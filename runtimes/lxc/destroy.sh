#!/usr/bin/env bash
# destroy.sh — TazPod LXC Destroy (CT 106, pet volume sopravvive)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TERRAFORM_DIR="${SCRIPT_DIR}/terraform"

cd "$TERRAFORM_DIR"
terraform init -input=false
terraform destroy -auto-approve -input=false
echo "CT 106 destroyed. Pet volume vm-999-disk-2 on CT 999 survives."
