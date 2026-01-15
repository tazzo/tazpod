#!/bin/bash
set -e

# --- DEVPOD-ZT: Zero Trust Environment Hardening (Bulletproof Edition) ---
SECRET_NAME="DEVPOD_PASSPHRASE_HASH"
USER_NAME="vscode"
SECRETS_DIR="/home/$USER_NAME/secrets"
VAULT_BASE="/home/$USER_NAME/.vault_ram"
VAULT_IMG="$VAULT_BASE/container.img"
VAULT_SIZE="256" # MB
MAPPER_NAME="zt_vault_$(hostname)"

cleanup() {
    unset PLAIN_PASS
    export INFISICAL_TOKEN=""
}
trap cleanup EXIT

# 0. IDEMPOTENZA
if mountpoint -q "$SECRETS_DIR" 2>/dev/null; then
    echo "✅ Environment already secured."
    exit 0
fi

if [[ ! -t 0 ]]; then exit 0; fi

echo "🔐 DEVPOD-ZT: Starting Secure Hardening..."

# 1. Login Infisical
infisical login --silent

# 2. Recupero Hash
HASH_VAL=$(infisical secrets get $SECRET_NAME --plain 2>/dev/null || true)

if [ -z "$HASH_VAL" ]; then
    echo "🆕 First-time setup detected."
    while true; do
        read -rs -p "Define Master Passphrase: " PLAIN_PASS
        echo
        read -rs -p "Confirm Master Passphrase: " PLAIN_PASS_CONFIRM
        echo
        [ "$PLAIN_PASS" = "$PLAIN_PASS_CONFIRM" ] && break
        echo "❌ Passwords do not match."
    done
    HASH_VAL=$(openssl passwd -6 -stdin <<< "$PLAIN_PASS")
    infisical secrets set $SECRET_NAME="$HASH_VAL" --silent > /dev/null
else
    echo "🔑 Secure credentials found."
    read -rs -p "Enter Master Passphrase to ENABLE ZERO TRUST: " PLAIN_PASS
    echo
    CHECK_HASH=$(openssl passwd -6 -salt "$(echo "$HASH_VAL" | cut -d'$' -f3)" -stdin <<< "$PLAIN_PASS")
    if [ "$CHECK_HASH" != "$HASH_VAL" ]; then
        echo "❌ WRONG PASSPHRASE."
        exit 1
    fi
fi

# 3. Blindatura Account
sudo chpasswd -e <<< "$USER_NAME:$HASH_VAL"
sudo chpasswd -e <<< "root:$HASH_VAL"

# 4. Loop Device Fix (0-31)
sudo mknod /dev/loop-control c 10 237 2>/dev/null || true
for i in $(seq 0 31); do
    sudo mknod /dev/loop$i b 7 $i 2>/dev/null || true
done

# 5. RAM Vault (LUKS)
echo "💾 Engaging Secure Enclave (RAM)..."
sudo mkdir -p "$VAULT_BASE"
if ! mountpoint -q "$VAULT_BASE"; then
    sudo mount -t tmpfs -o size=${VAULT_SIZE}M tmpfs "$VAULT_BASE"
fi

# --- GARBAGE COLLECTOR (Pulizia profonda) ---
echo "🧹 Cleaning up stale resources..."
sudo umount "$SECRETS_DIR" 2>/dev/null || true
if [ -e "/dev/mapper/$MAPPER_NAME" ]; then
    sudo cryptsetup close "$MAPPER_NAME" || true
fi
sudo losetup -a | grep "$VAULT_IMG" | cut -d: -f1 | xargs -r sudo losetup -d || true

# --- MONTAGGIO ---
if [ ! -f "$VAULT_IMG" ]; then
    echo "   Creating encrypted container..."
    sudo dd if=/dev/zero of="$VAULT_IMG" bs=1M count=$VAULT_SIZE status=none
    LOOP_DEV=$(sudo losetup -f --show "$VAULT_IMG")
    echo -n "$PLAIN_PASS" | sudo cryptsetup luksFormat "$LOOP_DEV" -
    echo -n "$PLAIN_PASS" | sudo cryptsetup open "$LOOP_DEV" "$MAPPER_NAME" -
    echo "   Formatting vault (ext4)..."
    sudo mkfs.ext4 -q "/dev/mapper/$MAPPER_NAME"
else
    echo "   Opening encrypted container..."
    LOOP_DEV=$(sudo losetup -f --show "$VAULT_IMG")
    echo -n "$PLAIN_PASS" | sudo cryptsetup open "$LOOP_DEV" "$MAPPER_NAME" -
fi

# Verifica se il filesystem esiste (nel caso di container pre-esistenti corrotti)
if ! sudo blkid "/dev/mapper/$MAPPER_NAME" | grep -q "ext4"; then
    echo "   ⚠️  No filesystem detected on vault. Formatting now..."
    sudo mkfs.ext4 -q "/dev/mapper/$MAPPER_NAME"
fi

sudo mkdir -p "$SECRETS_DIR"
echo "   Mounting enclave..."
sudo mount -t ext4 "/dev/mapper/$MAPPER_NAME" "$SECRETS_DIR"
sudo chown -R $USER_NAME:$USER_NAME "$SECRETS_DIR"

# 6. Export Segreti
echo "📦 Migrating secrets to Vault..."
infisical export --format=dotenv --silent > "$SECRETS_DIR/.env-infisical"
echo "✅ DEVPOD-ZT: Environment SECURED."
cleanup