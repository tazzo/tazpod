#!/usr/bin/env bash
# destroy.sh — TazPod LXC Destroy (CT 106, pet volume sopravvive)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TERRAFORM_DIR="${SCRIPT_DIR}/terraform"

cd "$TERRAFORM_DIR"
terraform init -input=false
# ssh_public_key has no default (create.sh supplies the provisioning machine's
# key); destroy ignores it, but Terraform still requires a value for every variable.
terraform destroy -auto-approve -input=false -var "ssh_public_key="
echo "CT 106 destroyed. Pet volume vm-999-disk-2 on CT 999 survives."
