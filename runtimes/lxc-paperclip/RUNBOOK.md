# RUNBOOK — CT108 `paperclip` (dedicated container for the agent company)

Run everything **from `tazlab` (`192.168.1.200`)**: the provisioning key is pinned
`from="192.168.1.200"`, so a workstation reaches it only through `ssh tazlab`.
Code lives in `tazpod/runtimes/lxc-paperclip/` (Terraform + `create.sh`/`destroy.sh`) and
`ansible/` (`role paperclip` = environment, `role paperclip-egress` = T4b chokepoint).

## 1. Provision the environment

```bash
cd /root/tazpod/runtimes/lxc-paperclip && ./create.sh
# or, convergently, as often as you like:
ANSIBLE_CONFIG=ansible/ansible.cfg ansible-playbook -i ansible/inventory.ini ansible/paperclip-baseline.yml
```

Address `192.168.1.208` (`terraform.tfvars` / `configs/runtime.env`; `create.sh` passes
`-e ansible_host=`). End state: tooling present, service account `paperclip` with **no sudo
and no SSH keys**, the container's own GPG key and gopass store on the data volume, the CLI
installed, the unit **installed but not started** — it is waiting for step 3.

## 2. Secrets the operator owns

The playbook seeds the agent store from your own store through stdin (nothing touches a
file, argv or a log). Two entries only you can create:

```bash
kubectl -n flux-system get secret paperclip-observer-token -o jsonpath='{.data.token}' \
  | base64 -d | gopass insert paperclip/k8s-token      # read-only cluster observation; then re-run
vault write -wrap-ttl=120s -force auth/approle-agent/role/paperclip-agent/secret-id
```

## 3. Carry the company (the only step that touches the old instance)

The database stays where it is — the container resumes against the same one. **Stop the old
service first**: two schedulers against one database is the failure the design forbids.

```bash
# on CT106, as root: freeze, then hand the instance tree to CT108
systemctl stop paperclip.service && systemctl disable paperclip.service   # keep intact for rollback
for p in secrets/master.key config.json data/storage data/run-logs; do
  rsync -aH --info=progress2 "/workspace/.paperclip/instances/default/$p" \
    192.168.1.208:/workspace/.paperclip/instances/default/
done
```

Nothing is regenerated for you, on purpose: the layer never runs `paperclipai onboard` (it
would mint a new instance id and a new `master.key`, orphaning the secrets the database
stores). Until those four artefacts exist the playbook says so and leaves the unit disabled.
Re-run it then — it fixes the root-owned files `rsync` produced, teaches the carried
`config.json` the new address, and starts the service.

## 4. Verify

```bash
ssh root@192.168.1.208 systemctl is-active paperclip.service
curl -fsS http://192.168.1.208:3100/api/health
# the secret path exactly as the unit runs it (it is also ExecStartPre)
ssh root@192.168.1.208 runuser -u paperclip -- env HOME=/workspace/home \
  GNUPGHOME=/workspace/home/.gnupg /usr/local/bin/paperclip-secrets-canary
# containment (T2)
ssh root@192.168.1.208 runuser -u paperclip -- sudo -n -l                            # fails
ssh root@192.168.1.208 runuser -u paperclip -- ssh -o BatchMode=yes root@192.168.1.200 true  # fails
```

Then the resumption set: `paperclipai doctor` clean, the UI shows the same company/agents/
issues/budgets, **one run binding a `secret_ref` succeeds** (that is the proof the carried
master key matches; the failure to watch for is `Secret decryption failed (master key
fingerprint: …)`), and no `Refusing to start against a stale schema`.

## 5. Roll back

Only one instance may run, so: stop the new one, start the old one.

```bash
ssh root@192.168.1.208 systemctl stop paperclip.service
pct exec 106 -- systemctl enable --now paperclip.service
```

CT108 can then be destroyed with `./destroy.sh`; the data volume follows the runtime's
preserve-the-disk pattern, so `/workspace` (master key, run logs, agent key) survives a
rebuild.

## 6. Routine

| Task | Action |
|---|---|
| Converge / dry run | re-run the playbook; add `--check --diff` |
| Rotate a secret | rotate it in your store, then on CT108: `gopass config --store agent core.readonly false`, `gopass insert -f agent/<path>`, `core.readonly true`, `systemctl restart paperclip` |
| Add an entry | extend `paperclip_gopass_entries` (and `paperclip_env_secrets` if the service needs it), flip `core.readonly` off, re-run, flip it back |
| Re-run the pi extension installs | `rm /workspace/home/.pi/.layer3-extensions-installed`, re-run |
| Upgrade a pinned tool | `rm` the binary/marker, re-run (the layer installs on version mismatch) |
| Alerts / a failed start | `journalctl -t paperclip-alert`; `journalctl -u paperclip -n 80`. Canary exit codes: 2 store missing, 3 recipient list not scoped, 4 key is not the container's own, 5 entry unreadable, 6 scoping broken, 7 not run in the service environment |

Deliberately absent, each for a reason recorded in the files: **no `paperclipai onboard`**
(step 3), **no Docker** (DESIGN §8.2 names a reachable `docker.sock` as an escape; the guest
runs with nesting off), **no operator keys or store**, **no store push remote** (its
credential cannot come from the store itself — add it when the repo exists:
`cd /workspace/gopass-agent && gopass git remote add origin <url>`), **no egress rules** here
(next section).

## Egress chokepoint (T4b) — operator steps

The role `paperclip-egress` runs as the second role of `paperclip-baseline.yml`, so the chokepoint is
provisioned with the container. These are only the steps the layer cannot do for you.

1. **Nothing to start by hand.** The role installs `mitmproxy` and the addon, generates its CA into
   `/usr/local/share/ca-certificates/paperclip-egress-ca.crt`, installs it in the container trust store,
   renders the policy to `/etc/paperclip-egress/policy.json`, loads the uid-split packet filter
   (`/etc/nftables.d/10-paperclip-egress.nft`) and enables + starts both units. The proxy listens on
   **`127.0.0.1:3128`**. The agent's proxy and CA variables arrive through the drop-in on
   `paperclip.service` — never through the agent's own configuration, which it could be talked out of.
2. **Host-side NIC posture (on `tazlab`).** The role stages the `/etc/pve/firewall/108.fw` content inside
   the guest (`templates/proxmox-firewall-108.fw.j2`). Copy it to the host and enable the datacenter
   firewall. Read its header first: on a single NIC the host **cannot** distinguish the chokepoint's
   traffic from the agent's, so that file is defence in depth; the enforceable split is the guest's
   uid-based ruleset.
3. **Verify the two properties that are the acceptance of T4b:**
   - *allowlist* — from the agent's context, a request to a host outside the policy must fail;
   - *injection* — a push to GitHub must succeed **with no token in the agent's environment**
     (`grep -c GITHUB_TOKEN /proc/$(systemctl show -p MainPID --value paperclip)/environ` → `0`), and the
     proxy log must show the injected request.
4. **Fail-closed check:** `systemctl stop paperclip-egress` must make the agent's outbound calls fail
   (the drop-in binds `paperclip.service` to the chokepoint, and the packet filter drops the agent uid's
   direct egress). Restart and confirm recovery.
5. **Rollback:** `systemctl disable --now paperclip-egress paperclip-egress-firewall`, remove the drop-in,
   `systemctl restart paperclip`. The agent returns to direct egress — and at that point the brokered
   credentials must be moved back inside the container, which is the "brokering deferred" state the
   design accepts **in writing**, not a silent fallback. `paperclip_egress_firewall_enabled: false` is
   the declarative equivalent for the packet-filter half.
