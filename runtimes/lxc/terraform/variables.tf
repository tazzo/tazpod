variable "proxmox_node" {
  type    = string
  default = "tazlab"
}

variable "ct_id" {
  type    = number
  default = 106
}

variable "hostname" {
  type    = string
  default = "tazpod-proxmox"
}

variable "ip_address" {
  type    = string
  default = "192.168.1.206"
}

variable "gateway" {
  type    = string
  default = "192.168.1.1"
}

variable "ssh_public_key" {
  type    = string
}

variable "cores" {
  type    = number
  default = 2
}

variable "memory_mb" {
  type    = number
  default = 2048
}

variable "swap_mb" {
  type    = number
  default = 512
}

variable "rootfs_size_gb" {
  type    = number
  default = 20
}

variable "lxc_template" {
  type = string
}
