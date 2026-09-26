output "ct_ip" {
  value = var.ip_address
}

output "ct_id" {
  value = var.ct_id
}

# The runtime facts file that create.sh and destroy.sh source (CT id, address,
# datastore, the data volume's name, the Proxmox host). Rendered from the same
# variables the guest is built from, so the scripts and the guest cannot drift;
# the checked-in copy carries the same values, so a re-apply is a no-op.
#
# There is deliberately NO inventory local_file here, although the operator
# runtime renders one: this runtime's inventory (ansible/inventory.ini, host alias
# `paperclip`) belongs to the provisioning layer, and create.sh passes the address
# to the playbook with -e ansible_host — two writers on that file would fight.
resource "local_file" "runtime_env" {
  content  = <<EOT
CT_ID=${var.ct_id}
HOSTNAME=${var.hostname}
IP_ADDRESS=${var.ip_address}
GATEWAY=${var.gateway}
STORAGE_POOL=${var.storage_pool}
DATA_VOLUME_NAME=vm-${var.ct_id}-disk-1
DATA_VOLUME_PATH=${var.data_volume_path}
PROXMOX_HOST=192.168.1.200
PROXMOX_NODE=${var.proxmox_node}
EOT
  filename = "${path.module}/../configs/runtime.env"
}
