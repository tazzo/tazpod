# TazPod: The Zero-Trust Containerized Developer Environment 🛡️📦

TazPod is an ephemeral, secure, and portable development environment built on **Docker**, **Go**, and **RAM-based Isolation**. It provides a fully configured IDE (Neovim, Tmux, Zellij) while ensuring that sensitive secrets are never exposed to the host filesystem or unauthorized processes through its **RAM Enclave (tmpfs)** architecture.

---

## 🚀 Key Features

*   **Zero Trust Architecture**: Secrets are stored in an AES-256-GCM encrypted vault (`vault.tar.aes`) and decrypted only into a volatile **RAM disk (tmpfs)**.
*   **AWS SSO Authentication**: Native integration with **AWS IAM Identity Center** for secure, token-based authentication (No static keys on disk).
*   **S3 Vault Sync**: Automated synchronization of your encrypted vault and nomadic identity to/from S3 (`tazlab-storage`, `eu-central-1`).
*   **AWS Enclave Bridge**: Securely bridges secrets from the RAM enclave to standard paths (e.g., `~/.aws`) via **Bind Mounts**.
*   **Modular Verticals**: Specialized images for AWS, Kubernetes, and AI-Enhanced development.
*   **Portable**: Single Go binary that works on any Linux machine with Docker.

---

## 📥 Installation

Install TazPod globally on your system using the official installer:

```bash
curl -sSL https://raw.githubusercontent.com/tazzo/tazpod/master/scripts/install.sh | bash
```
*Make sure `~/.local/bin` is in your `$PATH`.*

---

## 🏁 Project Initialization

To start using TazPod in a new or existing project, run:

```bash
tazpod init
```

This will create:
*   `.tazpod/vault/`: Storage for the local encrypted vault.
*   `.tazpod/config.yaml`: The main configuration file (AWS SSO profile, S3 bucket, image).

---

## ⚙️ Configuration (`config.yaml`)

The `.tazpod/config.yaml` file defines your environment's blueprint:

```yaml
version: 1.0
image: "tazzo/tazpod-k8s"
container_name: "tazpod-lab"
user: "tazpod"
aws_sso:
  start_url: "https://sso.example.com/start"
  account_id: "123456789012"
  role_name: "DeveloperAccess"
  region: "eu-central-1"
  profile: "tazlab-bootstrap"
```

---

## ☁️ Pre-compiled Images (Verticals)

| Image Name | Features |
| :--- | :--- |
| `tazzo/tazpod-base` | Ubuntu 24.04, Neovim, Tmux, Shell tools (Bash/Starship) |
| `tazzo/tazpod-aws` | Base + AWS CLI v2, SSO support |
| `tazzo/tazpod-k8s` | AWS + Kubectl, Helm, K9s, Talosctl, Stern, Terraform |
| `tazzo/tazpod-ai` | K8s + Gemini AI CLI, Pi Agent, Mnemosyne |

---

## 🎮 Usage Guide

### 1. Smart Entry (No Arguments)

Just run `tazpod` for the full automated bootstrap flow:
1.  **Init**: Setup project if missing.
2.  **Up**: Ensure container is running.
3.  **Bootstrap**: If vault is missing, perform `login` (SSO) → `pull vault` (S3).
4.  **Unlock**: Decrypt vault to RAM, setup bind mounts, and enter.

### 2. Vault & Sync Commands

| Command | Action |
| :--- | :--- |
| `tazpod login` | Authenticate with AWS SSO |
| `tazpod unlock` | Decrypt `vault.tar.aes` → tmpfs RAM disk |
| `tazpod lock` | Unmount RAM and wipe secrets |
| `tazpod pull vault` | Download latest vault from S3 |
| `tazpod push vault` | Upload local vault to S3 |
| `tazpod vpn up/down` | Start/Stop WireGuard tunnel (Tailscale planned) |

### 3. Background Sync

A daemon runs every 5 minutes while the vault is unlocked, performing an automatic `save` + `push vault` to ensure your state is always persisted to S3.

---
*For more technical details, see the [Documentation Hub](./docs/01-OVERVIEW.md).*
