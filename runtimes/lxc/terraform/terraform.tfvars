lxc_template   = "local:vztmpl/ubuntu-24.04-standard_24.04-2_amd64.tar.zst"
ssh_public_key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOLCfvDL4s7Qi8r4fQ7QsjpveYWV7VkCav7hCsjfT+DR"

# Runtime sizing (aligned with the live CT106 on 2026-09-10; dsh + Node needs the
# headroom, the original 2048/512 defaults were too tight)
memory_mb = 4096
swap_mb   = 1024
