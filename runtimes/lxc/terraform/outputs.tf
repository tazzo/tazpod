output "ct_ip" {
  value = var.ip_address
}

output "ct_id" {
  value = var.ct_id
}

resource "local_file" "ansible_inventory" {
  content = <<EOT
[tazpod]
${var.ip_address} ansible_user=root ansible_ssh_private_key_file=/root/.ssh/tazpod-provision
EOT
  filename = "${path.module}/../ansible/inventory.ini"
}

resource "local_file" "runtime_env" {
  content = <<EOT
CT_ID=${var.ct_id}
IP_ADDRESS=${var.ip_address}
HOSTNAME=${var.hostname}
EOT
  filename = "${path.module}/../configs/runtime.env"
}
