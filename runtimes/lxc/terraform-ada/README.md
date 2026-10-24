# terraform-ada — the Ada container

Creates the LXC guest that hosts the durable agent engine, and writes the inventory
and runtime env the provisioning scripts read. What runs **inside** is Ansible
(`../ansible/ada-baseline.yml`, role `ada`).

## What it creates

| | |
|---|---|
| CT ID | `110` (verify it is free: the API rejects a duplicate) |
| Hostname | `ada` |
| Address | `192.168.1.209/24`, gateway `192.168.1.1` |
| Cores | **6** (cpuset: 6 of the host's 8; TazPod keeps all 8) |
| RAM | **3072 MiB** + 1024 MiB swap |
| Disk | 30 GiB on `local-lvm` |
| Type | unprivileged, `nesting=1`, start on boot |
| No mount points | the engine's state is its own SQLite plus `/opt/ada/workspace` |

## Sizing, measured on 2026-10-24

From inside CT106 (which has no cgroup limits, so it reports the host):

- host: **8 cores**, **11 GiB RAM** (`MemTotal 11 534 336 kB`), **1 GiB swap**, ~9 GiB available;
- CT106: `memory.max = max`, `memory.high = max`, `cpuset 0-7` → **TazPod is unlimited today**.

Budget proposal (sum stays below the host, leaving ~2 GiB for the kernel and page cache):

| guest | RAM cap | swap | note |
|---|---|---|---|
| ada | 3072 | 1024 | the engine is Node + SQLite; the model calls happen off-box |
| tazpod (CT106) | **6144** | 1024 | today unlimited, measured ~2 GiB in use; a cap is a hot change |

The cap on CT106 is applied live with `pct set 106 --memory 6144` (no reboot: the
cgroup is rewritten). It can be lowered further or raised back at any time.

## Two host-side steps that Terraform cannot do

1. **`memory.high`** is not exposed by `pct` or the API. Proxmox sets it to 99.6 % of
   `memory.max` automatically, which is the failure mode that hit CT106 (a hard
   boundary with a 0.4 % warning band). To get a real soft boundary, edit the host
   file and restart that container:

   ```
   # /etc/pve/lxc/110.conf
   lxc.cgroup2.memory.high: 2576980377   # 80 % of 3072 MiB
   ```

2. **Verify the CT ID and the address** before applying: `110` and `.209` are the
   proposal; `.207`, `.210`, `.211` are taken (Proxmox MACs answer on the LAN).

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

`./ada-destroy.sh` removes the container **and its disk**.
