lxc_template = "local:vztmpl/ubuntu-24.04-standard_24.04-2_amd64.tar.zst"

# ssh_public_key is intentionally NOT set here: create.sh phase 1 passes
# -var "ssh_public_key=$(cat /root/.ssh/tazpod-provision.pub)" so a freshly
# created guest always gets the provisioning machine's current key. A hardcoded
# value here is how the orphan key of TD-063 (fingerprint
# SHA256:zPn1yMi/eP1aNK8NR8xvDDTevir9oXiZ9LRnkpLvfO90) ended up authorized on
# the node and inside CT106.

# Static address (not DHCP): the T4 credentials — Vault AppRole bound CIDRs, the
# Proxmox-side scoping — are written against this address, so it must be one the
# operator can guarantee. Keep 192.168.1.208 out of the router's DHCP pool.
ip_address = "192.168.1.208"
gateway    = "192.168.1.1"

# Target shape from the design: 4 cores, 6 GiB, 32 GiB rootfs, plus the guest's
# own 32 GiB data volume (rootfs is replaceable, the data volume is not).
cores               = 4
memory_mb           = 6144
swap_mb             = 1024
rootfs_size_gb      = 32
data_volume_size_gb = 32
