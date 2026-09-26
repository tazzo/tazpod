# T4 — the agent company's scoped Proxmox identity.
#
# APPLIED BY THE OPERATOR, never by create.sh. Creating a user, a role, an ACL or
# a token needs `Permissions.Modify` / `User.Modify` on `/access`, which stays
# operator-only (P2 §5) — that is the whole point of the split: the runtime's own
# apply uses the token it already has and does not touch access control.
#
# Every resource below is gated on var.manage_proxmox_access (default false), so
# with the shipped defaults this file contributes nothing to a plan. The operator
# runs, from this directory, with a token that holds Permissions.Modify:
#
#   terraform apply -var manage_proxmox_access=true
#   terraform output -raw proxmox_agent_token_id \
#     | gopass insert -f infra/paperclip/proxmox-token-id
#   terraform output -raw proxmox_agent_token_value \
#     | gopass insert -f infra/paperclip/proxmox-token-secret
#
# Two entries, not one: the egress chokepoint's credential list
# (`roles/paperclip-egress/defaults/main.yml`) injects `PVEAPIToken=<id>=<secret>`, so it
# consumes the halves separately — the id half is not a secret (it names the user and the
# token), the value half is.
#
# The two `/nodes/<node>` ACLs below are the one part this apply cannot do on its own: the
# bootstrap token holds Permissions.Modify where the pool lives but not on the node path, so
# the API answers `403 Permission check failed (/nodes/tazlab, Permissions.Modify)`. They are
# applied once by an operator with root on the node:
#   pveum acl modify /nodes/tazlab --roles PVEAuditor \
#     --users paperclip-agent@pve --tokens 'paperclip-agent@pve!agent' --propagate 0
#
# Order matters: the pool membership references the guest, so run ./create.sh
# first, then this apply.
#
# The token value is handled like this: PVE returns it exactly once, at creation,
# and the provider therefore keeps it in state (the `value` attribute is
# computed + sensitive). It is not written into any file in this repository and
# not into the guest — the egress proxy injects it (T4b), so the container only
# holds the proxy's address. The state file itself is root-only on the
# orchestration host and must be treated as secret material for this reason, which
# is the only copy path: the operator moves the value from state into gopass in
# the same window as the apply.
#
# create.sh and destroy.sh both refuse to run while one of these objects is in
# state, so a routine guest rebuild can never revoke the agent's identity as a
# side effect.

# The ACL anchor. Granting on the pool instead of per guest means a second agent
# guest later needs no new ACL, and no ACL ever reaches beyond the pool's members.
resource "proxmox_virtual_environment_pool" "agent" {
  count   = var.manage_proxmox_access ? 1 : 0
  pool_id = var.pve_access_pool
  comment = "Guests the Paperclip agent company may operate on its own (inspect, snapshot, power). Managed by tazpod/runtimes/lxc-paperclip (proxmox-access.tf)."
}

resource "proxmox_virtual_environment_pool_membership" "agent_guest" {
  count   = var.manage_proxmox_access ? 1 : 0
  pool_id = proxmox_virtual_environment_pool.agent[0].pool_id
  vm_id   = var.ct_id
}

# Exactly one custom role: the pool ACLs below are always a user ACL plus a token
# ACL, and with a privilege-separated token the effective privilege is the
# intersection of the two — so a second, narrower "audit only" role would either
# be dead code or would clip the token's power management. The built-in
# PVEAuditor supplies the node-level read that the inventory check needs.
resource "proxmox_virtual_environment_role" "guest_ops" {
  count   = var.manage_proxmox_access ? 1 : 0
  role_id = "PaperclipGuestOps"
  privileges = [
    "VM.Audit",     # read the guest's config and status — "inspect"
    "VM.PowerMgmt", # start/stop/shutdown/reset the guest's own lifecycle
    "VM.Snapshot",  # create and delete its own snapshots
  ]
  # Deliberately absent, and each for a reason:
  #   VM.Console / VM.Monitor — the console and termproxy calls must fail
  #     (PLAN T4 acceptance) and the agent has no PVE shell by design;
  #   VM.Config.*             — the guest's shape is git-declared, not
  #     agent-declared, and a config grant is a boundary change;
  #   VM.Snapshot.Rollback    — rolling the guest back is an operator decision;
  #   Datastore.* / Sys.Modify / Permissions.* — nothing the agent needs.
  # VM.Snapshot.Rollback is not part of VM.Snapshot in PVE, so the gap is real.
}

# No password, and no `keys`: there is nothing that authenticates *as* this user
# from a shell. The token is its only credential.
resource "proxmox_virtual_environment_user" "agent" {
  count   = var.manage_proxmox_access ? 1 : 0
  user_id = var.pve_agent_user
  comment = "Paperclip agent company (T4). Passwordless by design; scoped PVE token is its only credential. Managed by tazpod/runtimes/lxc-paperclip."
  # No expiration_date on the user: the expiry belongs on the credential. An
  # expiring account would silently disable the token at the same moment with no
  # renewal path, which is fail-closed for the wrong object.
}

resource "proxmox_virtual_environment_user_token" "agent" {
  count                 = var.manage_proxmox_access ? 1 : 0
  user_id               = proxmox_virtual_environment_user.agent[0].user_id
  token_name            = var.pve_agent_token_name
  expiration_date       = var.pve_agent_token_expiration
  privileges_separation = true
  comment               = "Scoped PVE token for the agent company (T4). Lives in gopass (infra/paperclip/proxmox-token) and is injected by the egress proxy — never inside the container. Rotation: bump pve_agent_token_name, apply, copy the new value to gopass, update the proxy."
  # privileges_separation = true is the property the design relies on: PVE caps
  # this token at the intersection of its own ACLs and the user's, so a leaked
  # token cannot act as the (passwordless) user.
}

# Guest operations, granted twice on purpose: with privilege separation the token
# needs its own ACL, and the user's ACL is the cap the intersection is taken
# against — granting only one of the two yields either a token that can do
# nothing or a user that is broader than the credential.
resource "proxmox_virtual_environment_acl" "pool_user" {
  count     = var.manage_proxmox_access ? 1 : 0
  path      = "/pool/${proxmox_virtual_environment_pool.agent[0].pool_id}"
  role_id   = proxmox_virtual_environment_role.guest_ops[0].role_id
  user_id   = proxmox_virtual_environment_user.agent[0].user_id
  propagate = true # pool ACLs must reach the members, not just the pool object
}

resource "proxmox_virtual_environment_acl" "pool_token" {
  count     = var.manage_proxmox_access ? 1 : 0
  path      = "/pool/${proxmox_virtual_environment_pool.agent[0].pool_id}"
  role_id   = proxmox_virtual_environment_role.guest_ops[0].role_id
  token_id  = proxmox_virtual_environment_user_token.agent[0].id
  propagate = true
}

# Inventory only: `/nodes/tazlab/status` must answer for the agent (PLAN T4
# acceptance) and that endpoint needs Sys.Audit, which PVEAuditor carries.
# propagate = false keeps it to exactly that path — no reach into /vms, /storage
# or /access, where the cluster's own ACLs live.
# The two `/nodes/<node>` ACLs are deliberately NOT managed here.
#
# They cannot be: creating an ACL on the node path needs Permissions.Modify *there*, the
# bootstrap token holds it only where the pool lives, and the API answers
#     403 Permission check failed (/nodes/tazlab, Permissions.Modify)
# so an apply that includes them always ends in a partial apply — the pool, role, user and
# token are created, these two fail, and every later apply repeats the failure. They are an
# operator step, once, with root on the node:
#
#   pveum acl modify /nodes/tazlab --roles PVEAuditor \
#     --users paperclip-agent@pve --tokens 'paperclip-agent@pve!agent' --propagate 0
#
# Inventory-only: `/nodes/<node>/status` needs Sys.Audit, which PVEAuditor carries, and
# `propagate 0` keeps it to exactly that path — no reach into /vms, /storage or /access.
# Verified with the agent's own token once the ACL was in place (T4 acceptance).

# The secret half *alone* — and this output exists because the provider's `value` is not it.
#
# Measured: `value` comes back as `<token id>=<secret>` (63 bytes for a 26-byte id and a 36-byte
# secret), so copying it into gopass whole makes every call answer **401** while looking perfectly
# plausible — the chokepoint injects `PVEAPIToken=<id>=<secret>`, and a doubled id authenticates
# nothing. The rotation procedure is:
#
#   terraform apply -var manage_proxmox_access=true        # after bumping pve_agent_token_name
#   terraform output -raw proxmox_agent_token_id     | gopass insert -f infra/paperclip/proxmox-token-id
#   terraform output -raw proxmox_agent_token_secret | gopass insert -f infra/paperclip/proxmox-token-secret
#
# Verified end to end on 2026-09-26: the extracted secret answers `200` on `/api2/json/version`,
# and `/api2/json/nodes/<node>/status` answers `403` until the *token's* ACL is granted (with
# `privsep = true` the user's ACL does not authorize the token's calls).
output "proxmox_agent_token_secret" {
  description = "Secret half of the agent's Proxmox token, with the provider's `<id>=` prefix removed."
  value       = var.manage_proxmox_access ? split("=", proxmox_virtual_environment_user_token.agent[0].value)[1] : ""
  sensitive   = true
}

output "proxmox_agent_user" {
  value = var.manage_proxmox_access ? proxmox_virtual_environment_user.agent[0].user_id : null
}

# Not a secret half: `<user>!<token>` is an identifier, and it is what the proxy
# uses as the username in the Authorization header.
output "proxmox_agent_token_id" {
  value = var.manage_proxmox_access ? proxmox_virtual_environment_user_token.agent[0].id : null
}

# Sensitive because this is the cleartext half, and it exists in exactly two
# places after the apply: this module's state and the provider's response. Read it
# only to insert it into gopass (command at the top of this file); it is printed
# nowhere else and is not part of the guest.
output "proxmox_agent_token_value" {
  value     = var.manage_proxmox_access ? proxmox_virtual_environment_user_token.agent[0].value : null
  sensitive = true
}
