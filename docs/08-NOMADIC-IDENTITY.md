# Nomadic Identity Synchronization ☁️🔄

TazPod "Nomadic Identity" allows you to carry your secure development environment across different machines (Laptop, Bastion, Cloud) using S3 as a persistence layer.

## 1. The Sync Scope

When you perform an identity sync, TazPod bundles the entire `.tazpod/` directory, which includes:
*   **The Encrypted Vault** (`vault.tar.aes`): All your secrets and session tokens.
*   **Configuration** (`config.yaml`, `secrets-sync-config.yml`).
*   **Gemini Memories** (`.gemini/`): Your AI assistant's history and learned context.

---

## 2. CLI Commands

### Push Identity (`tazpod push`)
Manually upload your current identity to S3.
```bash
tazpod push
# or
tazpod push identity
```

### Pull Identity (`tazpod pull`)
Download and restore your identity from S3.
```bash
tazpod pull
# or
tazpod pull identity
```

> **Note**: To sync Infisical secrets *after* pulling your identity, use `tazpod pull secrets`.

---

## 3. Automation

### Background Sync
When you start a TazPod session with `tazpod up`, a background daemon is spawned. Every 5 minutes, if the vault is unlocked, it automatically pushes your latest identity state to S3.

### Session Exit Sync
When you exit a `tazpod enter` session, TazPod performs a final `push` before locking the vault, ensuring no progress is lost.

---

## 4. Configuration

The sync logic targets the `tazlab-storage` bucket in the `eu-central-1` region by default. It uses standard AWS environment variables for authentication:
*   `AWS_ACCESS_KEY_ID`
*   `AWS_SECRET_ACCESS_KEY`

---
*Back to [Overview](../README.md)*
