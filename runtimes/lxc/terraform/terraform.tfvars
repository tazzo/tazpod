lxc_template   = "local:vztmpl/ubuntu-24.04-standard_24.04-2_amd64.tar.zst"

# ssh_public_key is intentionally NOT set here: create.sh phase 1 passes
# -var "ssh_public_key=$(cat ~/.ssh/id_ed25519.pub)" so a freshly created CT always
# gets the provisioning machine's current key. A hardcoded value here is how the
# orphan key of TD-063 (fingerprint SHA256:zPn1yMi/eP1aNK8NR8xvDDTeiv9oXiZ9LRnkpLvfO90)
# ended up authorized on the node and inside CT106.

# Runtime sizing (aligned with the live CT106 on 2026-09-10; dsh + Node needs the
# headroom, the original 2048/512 defaults were too tight)
memory_mb = 4096
swap_mb   = 1024
