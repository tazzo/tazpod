variable "proxmox_node" {
  type    = string
  default = "tazlab"
}

variable "ct_id" {
  type    = number
  default = 108
}

variable "hostname" {
  type    = string
  default = "paperclip"
}

# The guest's address is static (not DHCP, unlike CT106/CT107): the per-target
# credentials of T4 are CIDR-bound — the Vault AppRole's
# secret_id_bound_cidrs/token_bound_cidrs and the Proxmox-side ACL decisions are
# written against a known address — so a moving lease would make them unusable.
# The operator must keep this address OUT of the router's DHCP pool (exclusion or
# reservation), otherwise the guest can lose the address it is declared with and
# the CIDR bindings silently stop matching. create.sh's verify phase fails if the
# guest does not hold it.
variable "ip_address" {
  type    = string
  default = "192.168.1.208"
}

variable "gateway" {
  type    = string
  default = "192.168.1.1"
}

variable "storage_pool" {
  type    = string
  default = "local-lvm"
}

variable "bridge" {
  type    = string
  default = "vmbr0"
}

# The orchestration host's provisioning public key. No default on purpose:
# create.sh passes -var "ssh_public_key=$(cat /root/.ssh/tazpod-provision.pub)"
# so a freshly created guest is always reached with the key that exists *now*
# (the operator runtime carries the same rule after TD-063, where an orphan key
# stayed authorized because a stale value lived in terraform.tfvars).
variable "ssh_public_key" {
  type = string
}

variable "cores" {
  type    = number
  default = 4
}

variable "memory_mb" {
  type    = number
  default = 6144
}

variable "swap_mb" {
  type    = number
  default = 1024
}

variable "rootfs_size_gb" {
  type    = number
  default = 32
}

# The guest's own data volume. It is NOT the operator's pet volume
# (local-lvm:vm-999-disk-2 stays CT106's); PVE allocates this one in
# var.storage_pool and names it vm-<ct_id>-disk-1, mounted as mp0.
variable "data_volume_size_gb" {
  type    = number
  default = 32
}

variable "data_volume_path" {
  type    = string
  default = "/workspace"
}

# Empty means "allocate a fresh volume" (first build). create.sh passes the
# preserved volume's id ("local-lvm:vm-108-disk-1") when it finds the container
# absent but the volume still on the host — i.e. it comes back from a
# ./destroy.sh run — so the rebuild re-attaches the state instead of leaving it
# orphaned beside a new, empty volume.
variable "data_volume_id" {
  type    = string
  default = ""
}

variable "lxc_template" {
  type = string
}

# ─── T4: the agent's scoped Proxmox identity (proxmox-access.tf) ───
# Creating a user, a role, an ACL or a token needs Permissions.Modify/User.Modify
# on /access, which stays operator-only (P2 §5). Every resource in
# proxmox-access.tf is gated on this flag so the runtime's own apply never
# touches them; the operator flips it for the access apply only.
variable "manage_proxmox_access" {
  type        = bool
  default     = false
  description = "Operator-only gate for the PVE user/role/ACL/token declared in proxmox-access.tf."
}

variable "pve_agent_user" {
  type    = string
  default = "paperclip-agent@pve"
}

# Bumping the token name is the rotation path (PVE cannot re-issue a value for an
# existing token): apply, copy the new value out of state into gopass, update the
# egress proxy.
variable "pve_agent_token_name" {
  type    = string
  default = "agent"
}

# Hard expiry on the credential the agent's side uses (the user itself carries
# none). RFC3339, as the provider parses it.
variable "pve_agent_token_expiration" {
  type    = string
  default = "2027-03-31T00:00:00Z"
}

# The pool is the ACL anchor: ACLs on /pool/<id> reach every guest in the pool,
# so a second agent guest later needs no per-guest ACL.
variable "pve_access_pool" {
  type    = string
  default = "paperclip"
}
