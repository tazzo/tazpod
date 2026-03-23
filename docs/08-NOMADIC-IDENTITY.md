# Nomadic Identity Synchronization ☁️🔄

TazPod "Nomadic Identity" allows you to carry your secure development environment across different machines (Laptop, Bastion, Cloud) using S3 as a persistence layer.

## 1. The Sync Scope

When you perform a vault sync, TazPod bundles the encrypted `vault.tar.aes`, which includes:
*   **Secrets**: All decrypted files in `/home/tazpod/secrets`.
*   **Session Tokens**: Active AWS SSO sessions, cached passphrases.
*   **Dotfiles**: Configuration for tools bind-mounted into the vault (`.aws`, `.pi`, etc.).

---

## 2. CLI Commands

### Push Vault (`tazpod push vault`)
Manually upload your current vault state to S3.
```bash
tazpod push vault
```

### Pull Vault (`tazpod pull vault`)
Download and restore your vault from S3.
```bash
tazpod pull vault
```

---

## 3. Automation

### Background Sync
When you start a TazPod session with `tazpod up`, a background daemon is spawned. Every 5 minutes, if the vault is unlocked, it automatically pushes your latest vault state to S3.

### Session Exit Sync
When you exit a `tazpod enter` session, TazPod performs a final `push` before locking the vault, ensuring no progress is lost.

---

## 4. Configuration

The sync logic targets the `tazlab-storage` bucket in the `eu-central-1` region by default. It uses **AWS SSO** for authentication, leveraging the profile defined in `.tazpod/config.yaml`.

---
*Back to [Overview](../README.md)*
