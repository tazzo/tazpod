lxc_template = "local:vztmpl/ubuntu-24.04-standard_24.04-2_amd64.tar.zst"

# ssh_public_key is passed by ada-create.sh with the provisioning machine's current
# key, exactly like the TazPod module: never leave a stale key in this file.

# Sizing decisions (see README.md):
#   6 cores out of 8, 3 GiB RAM, 1 GiB swap, 30 GiB disk.
cores          = 6
memory_mb      = 3072
swap_mb        = 1024
rootfs_size_gb = 30
