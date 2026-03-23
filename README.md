# TazPod: The Zero-Trust Containerized Developer Environment 🛡️📦

TazPod is an ephemeral, secure, and portable development environment built on **Docker**, **Go**, and **Linux Namespaces**. It provides a fully configured IDE (Neovim, Tmux, Zellij) while ensuring that sensitive secrets are never exposed to the host filesystem or unauthorized processes through its unique **Ghost Mode**.

---

## 🚀 Key Features

*   **Zero Trust Architecture**: Secrets are stored in an AES-256-GCM encrypted vault (`vault.tar.aes`) mounted only within an isolated **Linux Namespace** ("Ghost Mode").
*   **Modular Verticals**: Choose between Base, AWS, K8s, or AI-Enhanced images.
*   **Infrastructure as Code**: Project settings are defined via `.tazpod/config.yaml`.
*   **AWS SSO Authentication**: Authenticate via `aws sso login` with a named profile stored in config.
*   **S3 Vault Sync**: Push and pull the encrypted vault and nomadic identity to/from S3 (`tazlab-storage`, `eu-central-1`).
*   **Portable**: Runs on any Linux machine with Docker.

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
*   `.tazpod/vault/`: Directory for the local encrypted vault.
*   `.tazpod/config.yaml`: The main configuration file (AWS SSO profile, S3 bucket, image).

---

## ⚙️ Configuration (`config.yaml`)

The `.tazpod/config.yaml` file defines how your environment behaves:

```yaml
version: 1.0
# The Docker image to use (see Pre-compiled Images)
image: "tazzo/tazpod-k8s"
container_name: "tazpod-lab"
user: "tazpod"
aws_sso:
  start_url: "https://sso.example.com/start"
  account_id: "123456789012"
  role_name: "DeveloperAccess"
  region: "us-east-1"
  profile: "tazlab-bootstrap"
features:
  ghost_mode: true # Enable Namespace isolation
  debug: false      # Show detailed logs
```

---

## ☁️ Pre-compiled Images (Verticals)

We provide several optimized images on Docker Hub:

| Image Name | Features |
| :--- | :--- |
| `tazzo/tazpod-base` | Ubuntu 24.04, Neovim, Tmux, Shell tools |
| `tazzo/tazpod-aws` | Base + AWS CLI v2, SSO support |
| `tazzo/tazpod-k8s` | AWS + Kubectl, Helm, K9s, Talosctl, Stern |
| `tazzo/tazpod-ai` | K8s + Gemini AI CLI for assisted coding |

---

## 🎮 Usage Guide

### 1. Smart Entry (No Arguments)

The primary way to launch TazPod. Just run:

```bash
tazpod
```

This triggers the **smartEntry** bootstrap flow:
1.  Auto-initializes the project if `.tazpod/` does not exist.
2.  Ensures the container is running (`up`).
3.  If no vault is present locally, runs the full bootstrap sequence:
    *   `login` → authenticate via AWS SSO
    *   `pull vault` → download `vault.tar.aes` from S3
    *   `unlock` → decrypt vault, mount tmpfs (64MB), bind-mount `.aws`, set up identity
4.  Enters the container shell.

### 2. Secrets & AWS SSO

TazPod manages secrets through an AES-256-GCM encrypted vault synced to S3.

**Authentication:**
```bash
tazpod login       # Run aws sso login --profile <profile from config.yaml>
```

**Vault lifecycle:**
```bash
tazpod pull vault     # Download vault.tar.aes from S3 (tazpod/vault/vault.tar.aes)
tazpod unlock         # Decrypt vault → tmpfs → bind-mount .aws → setup identity
tazpod lock           # Unmount .aws bind + unmount tmpfs (RAM zeroed)
tazpod save           # Re-encrypt current vault state → vault.tar.aes (AES-256-GCM)
tazpod push vault     # Upload vault.tar.aes to S3
```

**Nomadic identity:**
```bash
tazpod pull identity  # Download identity tarball from S3 (tazpod/identities/global.tar.gz)
tazpod push identity  # Upload identity tarball to S3
```

A **sync daemon** runs every 5 minutes and automatically performs `save` + `push vault` while the vault is unlocked.

### 3. CLI Command Reference

| Command | Description |
| :--- | :--- |
| `tazpod` | Smart entry (bootstrap + enter) |
| `tazpod up` | Start the container |
| `tazpod down` | Stop and remove the container |
| `tazpod enter` | Open a shell inside the container |
| `tazpod unlock` | Decrypt vault into RAM |
| `tazpod lock` | Unmount RAM and lock vault |
| `tazpod login` | Authenticate with AWS SSO |
| `tazpod pull vault` | Download vault from S3 |
| `tazpod push vault` | Upload vault to S3 |
| `tazpod vpn [up\|down]` | Start/Stop WireGuard VPN |
