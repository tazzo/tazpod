# terraform-ada — the Ada container

Creates the LXC guest that hosts the durable agent engine, and writes the inventory
and runtime env the provisioning scripts read. What runs **inside** is Ansible
(`../ansible/ada-baseline.yml`, role `ada`).

## What it creates

| | |
|---|---|
| CT ID | `110` (created 2026-10-24) |
| Hostname | `ada` |
| Address | `192.168.1.209/24`, gateway `192.168.1.1` |
| Cores | **6** (cpuset: 6 of the host's 8; TazPod keeps all 8) |
| RAM | **3072 MiB** + 1024 MiB swap |
| Disk | 30 GiB on `local-lvm` |
| Type | unprivileged, `nesting=1`, start on boot |
| Mount | **`/home/ada`** = `local-lvm:vm-998-disk-1`, a 40 GiB pet volume owned by **CT 998** (protected), created by `../terraform-ada-pet` |

## Sizing, measured on 2026-10-24

From the Proxmox API (a reading from inside CT106 is misleading: its cgroup files show `max` but
the guest is capped and its `MemTotal` is the cap):

- host: AMD Ryzen 7 5825U, **8 cores / 16 threads**, **30.8 GiB RAM**, 12.7 GiB free, 8 GiB swap;
- CT106: **8 cores**, no cpulimit, **11264 MiB**, swap 1024, `keyctl+nesting`;
- other guests: CT100 1.5 GiB, CT107 1 GiB, CP 8 GiB, worker 8 GiB, pet 256 MiB.

Budget proposal (sum stays below the host, leaving ~2 GiB for the kernel and page cache):

| guest | RAM cap | swap | note |
|---|---|---|---|
| ada | 3072 | 1024 | the engine is Node + SQLite; the model calls happen off-box |
| tazpod (CT106) | **unchanged 11264** | 1024 | no cap needed while 12.7 GiB are free |

If the host ever starts swapping, a cap on CT106 is a hot change:
`pct set 106 --memory 8192` (no reboot: the cgroup is rewritten).

## Two host-side steps that Terraform cannot do

1. **`memory.high`** is not exposed by `pct` or the API. Proxmox sets it to 99.6 % of
   `memory.max` automatically, which is the failure mode that hit CT106 (a hard
   boundary with a 0.4 % warning band). To get a real soft boundary, edit the host
   file and restart that container:

   ```
   # /etc/pve/lxc/110.conf
   lxc.cgroup2.memory.high: 2576980377   # 80 % of 3072 MiB
   ```

2. **CT ID and address are now fixed**: `110` and `.209` (`.207`, `.210`, `.211` were
   taken by Proxmox guests on the LAN). `provision_key_path` names the private key the
   generated Ansible inventory uses.

## SSH access

| | |
|---|---|
| from the provisioning host | `ssh ada` (key `infra/ada/ssh/private`, `~/.ssh/ada`, config block `Host ada`) |
| from a phone/tablet on the LAN or VPN | `ssh root@192.168.1.209`, password `gopass show -o infra/ada/ssh/password` |
| as the service user | `ssh -l ada ada` (home `/home/ada`) |

Both key and password are allowed. The Ansible role installs the key, sets the root password from a
sha512 hash, writes `/etc/ssh/sshd_config.d/10-ada.conf`, and **keeps the host keys on the pet
volume** (`/home/ada/state/ssh-host-keys`): a recreated container restores its own identity, so
clients do not see a changed host key.

## Recreating the container

`./ada-create.sh` is the whole procedure: `terraform apply`, wait for SSH, rsync the engine, run the
role (which installs dependencies, SSH and the service), health check. A bare
`terraform apply -replace=proxmox_virtual_environment_container.ada` also works — the role restores
the host keys, the access key and the password — but the engine code has to be rsynced again.

## Apply

```
# as root, with gopass unlocked
./ada-create.sh
```

It runs `terraform apply`, waits for SSH, copies the engine, runs the Ansible role,
installs the npm dependencies and waits for `/health`. Secrets come from gopass and
never touch git:

| variable | gopass path |
|---|---|
| Proxmox API token | `bootstrap/proxmox/token-id` + `token-secret` |
| `ada_opencode_api_key` | `infra/t3code/opencode-key` |
| `ada_exa_api_key` | `infra/exa/api-key` |
| `ada_http_token` | `infra/ada/http-token` (create it first) |

`./ada-destroy.sh` removes the container and its rootfs. **The data is not there**: it lives on the
pet volume owned by CT 998, which stays. Changing the `mount_point` block **recreates** the
container — the provider replaces a container whose mount configuration changes.
