locals {
  # Empty data_volume_id means "let PVE allocate" (first build); a value means
  # "attach this existing volume" (rebuild after destroy.sh preserved it).
  data_volume_volume = var.data_volume_id != "" ? var.data_volume_id : var.storage_pool
}

resource "proxmox_virtual_environment_container" "paperclip" {
  node_name = var.proxmox_node
  vm_id     = var.ct_id

  # DESIGN decision A: a hardened unprivileged LXC, not a VM. Kernel-sharing
  # residual risk is accepted and recorded in the design; it is not an oversight.
  # AppArmor is left at PVE's default profile for unprivileged containers — there
  # is no Terraform attribute for it, and create.sh's raw-config phase asserts the
  # profile was not set to `unconfined` rather than letting a weakened guest boot
  # and then provisioning it.
  unprivileged = true

  # The company's instance is a service: after a host reboot it has to come back
  # without an operator shell (the storage runtime sets the same flag for the
  # same reason; the operator runtime leaves the default).
  start_on_boot = true

  initialization {
    hostname = var.hostname

    # Static address, gateway from the same variables the runtime facts file is
    # rendered from. See var.ip_address for why this guest is not on DHCP: the
    # T4 credentials are CIDR-bound to this address.
    ip_config {
      ipv4 {
        address = "${var.ip_address}/24"
        gateway = var.gateway
      }
    }

    user_account {
      # The ONLY key in this guest: the orchestration host's provisioning public
      # key (/root/.ssh/tazpod-provision.pub). Its private half never leaves the
      # orchestration host, so nothing placed here opens the hypervisor — and no
      # operator personal key (the operator's ~/.ssh/id_ed25519) is placed here,
      # by design: the agent company's container carries no operator identity.
      # Keep this list to exactly one entry; a stale second entry is how TD-063's
      # orphan key survived a rebuild.
      keys = [var.ssh_public_key]
    }
  }

  network_interface {
    name = "eth0"
    # Bridge from the variable rather than the provider default, so the runtime
    # facts file and the guest agree on the segment.
    bridge = var.bridge
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
    datastore_id = var.storage_pool
    size         = var.rootfs_size_gb
  }

  # No `features {}` block: nesting and keyctl are both off by omission.
  # nesting was enabled in the first draft because the sibling runtime sets it — but
  # that runtime installs Docker and this one does not (the agent's images are built
  # by GitHub Actions and delivered through Flux), and the confinement review
  # (P7 §4.3) lists `nesting=1` as a cost to pay only against a hard requirement:
  # it exposes the host's procfs/sysfs to the guest. Reintroducing it means adding
  # the block back *with* a reason and a test, not copying a default.
  # keyctl() stays off for the same reason: this guest's key path is the
  # passphrase-less scoped store key (T3), which does not use the kernel keyring.

  # The guest's OWN data volume. It is not the operator's pet volume
  # (local-lvm:vm-999-disk-2 stays CT106's /workspace), and it is not a bind mount
  # from the host either: PVE names the container's rootfs vm-<id>-disk-0 and the
  # first storage-backed mount point vm-<id>-disk-1, attached as mp0 here.
  mount_point {
    volume = local.data_volume_volume
    # `size` is only meaningful while PVE allocates the volume; when attaching an
    # already-existing one it must be null, otherwise the apply asks for a resize
    # of a volume whose size is already the fact (the operator runtime's
    # pet-backed mount declares no size either).
    size = var.data_volume_id == "" ? "${var.data_volume_size_gb}G" : null
    path = var.data_volume_path
    # The volume holds the carried instance state — master key, instance tree,
    # run logs — which is the one thing a lost container cannot be rebuilt from.
    # Excluding it from guest backups would make ./destroy.sh unrecoverable.
    backup = true
  }

  lifecycle {
    ignore_changes = [
      # Kept from the operator runtime. PVE can re-resolve the template path on
      # its own after a template update, and a diff on this attribute is not a
      # change anyone asked for — without the guard it would replace the guest and
      # thereby detach the data volume.
      operating_system[0].template_file_id,
    ]
  }
}
