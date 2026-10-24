variable "proxmox_node" {
  type    = string
  default = "tazlab"
}

variable "ct_id" {
  type    = number
  default = 110
}

variable "hostname" {
  type    = string
  default = "ada"
}

variable "ip_address" {
  type    = string
  default = "192.168.1.209"
}

variable "gateway" {
  type    = string
  default = "192.168.1.1"
}

# Where the provisioning machine keeps the private key that matches ssh_public_key.
variable "provision_key_path" {
  type    = string
  default = "/root/.ssh/tazpod-provision"
}

variable "ssh_public_key" {
  type = string
}

# CPU: the container sees 6 of the host's 8 cores. No cpulimit is set, so when the
# other guest is idle it can use all 6; under contention the kernel fair scheduler
# shares them (cpuunits stays at the default weight).
variable "cores" {
  type    = number
  default = 6
}

# RAM: 3 GiB is the working set of the engine plus its commands; the model calls
# happen off-box. The host has ~11 GiB and CT106 is capped separately.
variable "memory_mb" {
  type    = number
  default = 3072
}

variable "swap_mb" {
  type    = number
  default = 1024
}

variable "rootfs_size_gb" {
  type    = number
  default = 30
}

variable "lxc_template" {
  type = string
}
