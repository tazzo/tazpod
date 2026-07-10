
resource "proxmox_virtual_environment_container" "tazpod" {
  node_name    = var.proxmox_node
  vm_id        = var.ct_id
  unprivileged = true

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
    # keyctl is set via API raw_config phase (requires root@pam, API token can't set it)
  }

  # Mount persistente da pet CT 999 — size omesso per evitare ForceNew
  mount_point {
    volume = "local-lvm:vm-999-disk-2"
    path   = "/workspace"
  }

  lifecycle {
    ignore_changes = [
      operating_system[0].template_file_id,
    ]
  }
}
