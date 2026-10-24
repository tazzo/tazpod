terraform {
  required_version = ">= 1.5.0"
  required_providers {
    proxmox = {
      source  = "bpg/proxmox"
      version = ">= 0.108.0"
    }
  }
}

provider "proxmox" {
  endpoint = "https://192.168.1.200:8006/"
  insecure = true
}

# The pet volume of Ada, in the same shape as CT 999 for TazPod: a small container
# owns the disk, the working container mounts it by ID. Destroying the working
# container therefore does not touch the data, and a host power loss only means a
# reattach at boot.
resource "proxmox_virtual_environment_container" "ada_pet" {
  node_name    = "tazlab"
  vm_id        = 998
  unprivileged = true
  protection   = true
  description  = "PET STORAGE for Ada — DO NOT DELETE. Owns vm-998-disk-1, mounted at /home/ada in CT 110."

  initialization {
    hostname = "ada-pet"
    ip_config {
      ipv4 {
        address = "dhcp"
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

  # The volume that becomes Ada's home. Never mounted here: it belongs to CT 110.
  mount_point {
    volume = "local-lvm"
    size   = "40G"
    path   = "/mnt/ada-home"
    backup = true
  }
}

variable "ssh_public_key" {
  type = string
}

output "pet_id" {
  value = 998
}

output "home_volume" {
  value = "local-lvm:vm-998-disk-1"
}
