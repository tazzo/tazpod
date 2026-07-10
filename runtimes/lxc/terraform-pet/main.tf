terraform {
  required_version = ">= 1.5.0"
  required_providers {
    proxmox = {
      source  = "bpg/proxmox"
      version = ">= 0.94.0"
    }
  }
}

provider "proxmox" {
  endpoint = "https://192.168.1.200:8006/"
  insecure = true
}

resource "proxmox_virtual_environment_container" "pet_storage" {
  node_name    = "tazlab"
  vm_id        = 999
  unprivileged = true
  protection   = true
  description  = "PET STORAGE — DO NOT DELETE. Hosts persistent volumes for cattle containers."

  initialization {
    hostname = "pet-storage"
    ip_config {
      ipv4 {
        address = "dhcp"
      }
    }
    user_account {
      keys = [trimspace(file("~/.ssh/id_ed25519.pub"))]
    }
  }

  network_interface {
    name = "eth0"
  }

  operating_system {
    template_file_id = "local:vztmpl/debian-12-standard_12.12-1_amd64.tar.zst"
    type             = "debian"
  }

  cpu {
    cores = 1
  }

  memory {
    dedicated = 256
  }

  disk {
    datastore_id = "local-lvm"
    size         = 2
  }

  features {
    nesting = true
  }

  # Persistent volume — owned by pet, mountable by any cattle container
  mount_point {
    volume = "local-lvm"
    size   = "10G"
    path   = "/mnt/hermes-volume"
    backup = true
  }
}

output "pet_id" {
  value = 999
}

output "pet_volume" {
  value = "local-lvm:vm-999-disk-1"
}
