#!/usr/bin/env bash
# ada-secrets-init.sh — one-time (and idempotent) migration of Ada's secrets from the
# environment file into age-encrypted files in her home.
#
# Everything stays inside the container: the identity and the encrypted secrets live in
# /home/ada/.secrets, so a backup of the home is complete and portable. The plaintext
# never touches a disk: gopass -> pipe -> age -> .age file.
#
# Run from the provisioning host with gopass unlocked:
#   ./ada-secrets-init.sh
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SSH=(ssh -o BatchMode=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i "${ADA_PROVISION_KEY:-/root/.ssh/tazpod-provision}" root@192.168.1.209)
SECRETS=/home/ada/.secrets

# name in the container  ->  gopass path
declare -A MAP=(
  [OPENCODE_API_KEY]="infra/t3code/opencode-key"
  [EXA_API_KEY]="infra/exa/api-key"
  [AGENTMAIL_API_KEY]="infra/agentmail/api-key"
  [ADA_HTTP_TOKEN]="infra/ada/http-token"
)

echo "── identity"
"${SSH[@]}" "sudo -u ada -H bash -lc 'cd $SECRETS && { [ -f identity ] || age-keygen -o identity; } && chmod 600 identity && age-keygen -y identity > recipients.txt && chmod 644 recipients.txt'"
RECIPIENT="$("${SSH[@]}" "cat $SECRETS/recipients.txt")"
IDENTITY="$("${SSH[@]}" "cat $SECRETS/identity")"
printf '%s\n' "$IDENTITY" | gopass insert -f infra/ada/secrets/age-identity >/dev/null
printf '%s\n' "$RECIPIENT" | gopass insert -f infra/ada/secrets/age-recipient >/dev/null
echo "   public key: $RECIPIENT"
echo "   identity and recipient stored in gopass (infra/ada/secrets/…)"

echo "── encrypting"
for name in "${!MAP[@]}"; do
  value="$(gopass show -o "${MAP[$name]}")"
  if [ -z "$value" ]; then
    echo "   SKIP $name: empty value in gopass ${MAP[$name]}"
    continue
  fi
  printf '%s' "$value" | "${SSH[@]}" "sudo -u ada -H tee $SECRETS/$name.age.tmp >/dev/null && sudo -u ada -H age -R $SECRETS/recipients.txt -o $SECRETS/$name.age $SECRETS/$name.age.tmp && rm -f $SECRETS/$name.age.tmp && chmod 600 $SECRETS/$name.age"
  echo "   $name -> $SECRETS/$name.age"
done

echo "── restart"
"${SSH[@]}" "systemctl reset-failed ada; systemctl restart ada"
sleep 4
"${SSH[@]}" "systemctl is-active ada; tr '\0' '\n' < /proc/\$(systemctl show -p MainPID --value ada)/environ | grep -cE '^(OPENCODE_API_KEY|EXA_API_KEY|AGENTMAIL_API_KEY|ADA_HTTP_TOKEN)=.+' || echo 'no secrets in env'"
echo "── files in $SECRETS"
"${SSH[@]}" "ls -l $SECRETS"
