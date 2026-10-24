resource "proxmox_virtual_environment_container" "ada" {
  node_name    = var.proxmox_node
  vm_id        = var.ct_id
  unprivileged = true

  description = "Ada — the durable agent house (pi-durable engine, named agents). Managed by runtimes/lxc/terraform-ada."
  tags        = ["ada", "agents"]

  initialization {
    hostname = var.hostname

    ip_config {
      ipv4 {
        address = "${var.ip_address}/24"
        gateway = var.gateway
      }
    }

    user_account {
      keys = [var.ssh_public_key]
    }
  }

  network_interface {
    name = "eth0"
  }

  operating_system {
    template_file_id = var.lxc_template
    type             = "ubuntu"
  }

  cpu {
    cores = var.cores
  }

  memory {
    dedicated = var.memory_mb
    swap      = var.swap_mb
  }

  disk {
    datastore_id = "local-lvm"
    size         = var.rootfs_size_gb
  }

  features {
    nesting = true
  }

  started       = true
  start_on_boot = true

  lifecycle {
    ignore_changes = [
      operating_system[0].template_file_id,
    ]
  }
}
