#!/bin/bash

# infisical-unlock.sh
# Universal Zero-Trust Unlocker (Battery Included Version)
# Usage: infisical-unlock.sh secrets.yml

CONFIG_FILE=$1
SECRET_DIR="/dev/shm/secrets"
ENV_FILE="$HOME/.tazlab-env"

if [ -z "$CONFIG_FILE" ]; then
  echo "❌ Usage: unlock <config-file.yml>"
  exit 1
fi

if [ ! -f "$CONFIG_FILE" ]; then
  echo "❌ Error: Configuration file $CONFIG_FILE not found."
  exit 1
fi

echo "🔐 == Zero-Trust Unlock Session =="

# 1. Login obbligatorio (più sicuro e affidabile)
echo "🔑 Login to Infisical EU..."
infisical login

# 2. Setup Secret Directory in RAM (SHM)
mkdir -p "$SECRET_DIR"
chmod 700 "$SECRET_DIR"

# 3. Process Secrets using yq
echo "🚀 Downloading secrets to RAM storage ($SECRET_DIR)..."
echo "# Generated Cluster Environment" >"$ENV_FILE"

# Estraiamo i dati dal file YAML e cicliamo
# Formato: name,file,env
while IFS=',' read -r name file env_var; do
  [ -z "$name" ] || [ "$name" == "null" ] && continue

  echo "  -> Fetching $name..."
  infisical secrets get "$name" --plain --silent >"$SECRET_DIR/$file" 2>/tmp/infisical_err

  if [ -s "$SECRET_DIR/$file" ]; then
    chmod 600 "$SECRET_DIR/$file"
    echo "export $env_var=\"$SECRET_DIR/$file\"" >>"$ENV_FILE"
  else
    echo "  ⚠️ Failed to fetch $name (Check /tmp/infisical_err)"
  fi
done < <(yq e '.secrets[] | [.name, .file, .env] | join(",")' "$CONFIG_FILE")

echo "✅ Unlock complete."
echo "🔗 Run 'source ~/.tazlab-env' or type 'unlock' to activate."

