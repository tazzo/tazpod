#!/bin/bash
set -e

# --- ZERO TRUST BOOTSTRAP ---
SECRET_NAME="DEVPOD_PASSPHRASE_HASH"
USER_NAME="vscode"
SECRETS_DIR="/home/$USER_NAME/secrets"
VAULT_IMG="/home/$USER_NAME/.vault_ram/container.img"
VAULT_SIZE="512M" # Dimensione RAM Disk Cifrato

# Funzione per pulire variabili sensibili alla chiusura
cleanup() {
    unset PLAIN_PASS
    unset INFISICAL_TOKEN
}
trap cleanup EXIT

echo "🔒 --- SECURE BOOTSTRAP INITIATED ---"

# 1. Verifica Infisical
if ! command -v infisical &> /dev/null; then
    echo "❌ Errore: Infisical CLI non trovata."
    exit 1
fi

# 2. Gestione Hash Passphrase
echo "🔍 Interrogazione Infisical Vault..."
HASH_VAL=$(infisical secrets get $SECRET_NAME --plain 2>/dev/null || true)

if [ -z "$HASH_VAL" ]; then
    echo "⚠️  Nessun hash trovato. È il primo avvio?"
    echo "🆕 Crea una PASSPHRASE robusta per blindare questo DevPod."
    echo "   (Questa passphrase servirà per sudo, ssh e per decifrare i segreti)"
    
    while true; do
        read -rs -p "Inserisci Nuova Passphrase: " PLAIN_PASS
        echo
        read -rs -p "Conferma Passphrase: " PLAIN_PASS_CONFIRM
        echo
        [ "$PLAIN_PASS" = "$PLAIN_PASS_CONFIRM" ] && break
        echo "❌ Le password non coincidono. Riprova."
    done

    # Genera Hash SHA-512 (compatibile con /etc/shadow)
    HASH_VAL=$(openssl passwd -6 -stdin <<< "$PLAIN_PASS")
    
    # Salva su Infisical
    infisical secrets set $SECRET_NAME="$HASH_VAL" > /dev/null
    echo "✅ Hash salvato su Infisical."
else
    echo "✅ Hash recuperato da Infisical."
    # Chiediamo la passphrase per sbloccare il disco (l'hash serve solo per l'utente Linux)
    read -rs -p "🔑 Inserisci Passphrase per sbloccare il sistema: " PLAIN_PASS
    echo
    
    # Verifica rudimentale (hashiamo e confrontiamo)
    CHECK_HASH=$(openssl passwd -6 -salt "$(echo "$HASH_VAL" | cut -d'$' -f3)" -stdin <<< "$PLAIN_PASS")
    if [ "$CHECK_HASH" != "$HASH_VAL" ]; then
        echo "❌ Passphrase Errata! Accesso Negato."
        sleep 3
        exit 1
    fi
fi

# 3. Blinda Utente e Root
echo "🛡️  Impostazione password di sistema..."
echo "$USER_NAME:$HASH_VAL" | sudo chpasswd -e
echo "root:$HASH_VAL" | sudo chpasswd -e

# 4. Nuclear Option: Rimuovi accesso SSH senza password
echo "🚫 Rimozione chiavi SSH iniettate (Richiede password ad ogni accesso)..."
rm -f /home/$USER_NAME/.ssh/authorized_keys

# 5. RAM Vault (LUKS)
echo "💾 Preparazione RAM Vault ($SECRETS_DIR)..."
mkdir -p $(dirname $VAULT_IMG)
mkdir -p $SECRETS_DIR

# Monta tmpfs se non esiste
if ! mountpoint -q $(dirname $VAULT_IMG); then
    sudo mount -t tmpfs -o size=${VAULT_SIZE} tmpfs $(dirname $VAULT_IMG)
fi

# Crea immagine disco se non esiste
if [ ! -f "$VAULT_IMG" ]; then
    echo "   Creazione container cifrato..."
    dd if=/dev/zero of="$VAULT_IMG" bs=1M count=200 status=none
    echo -n "$PLAIN_PASS" | sudo cryptsetup luksFormat "$VAULT_IMG" -
    echo -n "$PLAIN_PASS" | sudo cryptsetup open "$VAULT_IMG" secrets_vault -
    sudo mkfs.ext4 /dev/mapper/secrets_vault > /dev/null
    sudo mount /dev/mapper/secrets_vault "$SECRETS_DIR"
    sudo chown -R $USER_NAME:$USER_NAME "$SECRETS_DIR"
    echo "✅ Vault Creato e Montato."
else
    echo "   Montaggio container esistente..."
    # Se il container esiste già (es. riavvio senza distruzione), proviamo ad aprirlo
    if [ ! -e "/dev/mapper/secrets_vault" ]; then
         echo -n "$PLAIN_PASS" | sudo cryptsetup open "$VAULT_IMG" secrets_vault -
         sudo mount /dev/mapper/secrets_vault "$SECRETS_DIR"
    fi
    echo "✅ Vault Rimontato."
fi

# Pulizia finale memoria
unset PLAIN_PASS
echo "🚀 Sistema Blindato. Buona lavoro."
