# TazPod: The Zero-Trust Containerized Developer Environment 🛡️📦

TazPod is an ephemeral, secure, and portable development environment built on **Docker**, **Go**, and **Linux Namespaces**. It provides a fully configured IDE (Neovim, Tmux, Zellij) while ensuring that sensitive secrets are never exposed to the host filesystem or unauthorized processes through its unique **Ghost Mode**.

---

## 🚀 Key Features

*   **Zero Trust Architecture**: Secrets are stored in a LUKS-encrypted vault (`vault.img`) mounted only within an isolated **Linux Namespace** ("Ghost Mode").
*   **Modular Verticals**: Choose between Base, Infisical, K8s, or AI-Enhanced images.
*   **Infrastructure as Code**: Project settings are defined via `.tazpod/config.yaml`.
*   **Infisical Native**: Securely pull secrets from Infisical with persisted authentication sessions *inside* the encrypted vault.
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
*   `.tazpod/config.yaml`: The main configuration file.
*   `.tazpod/Dockerfile`: A template to extend the environment.
*   `.tazpod/secrets-sync-config.yml`: A mapping file for your Infisical secrets.

---

## ⚙️ Configuration (`config.yaml`)

The `.tazpod/config.yaml` file defines how your environment behaves:

```yaml
version: 1.0
# The Docker image to use (see Pre-compiled Images)
image: "tazzo/tazlab.net:tazpod-k8s"
container_name: "tazpod-lab"
user: "tazpod"
features:
  ghost_mode: true # Enable Namespace isolation
  debug: false      # Show detailed logs
```

---

## ☁️ Pre-compiled Images (Verticals)

We provide several optimized images on Docker Hub:

| Image Name | Features |
| :--- | :--- |
| `tazzo/tazlab.net:tazpod-base` | Ubuntu 24.04, Neovim, Tmux, Shell tools |
| `tazzo/tazlab.net:tazpod-infisical` | Base + Infisical CLI for secret management |
| `tazzo/tazlab.net:tazpod-k8s` | Infisical + Kubectl, Helm, K9s, Talosctl, Stern |
| `tazzo/tazlab.net:tazpod-gemini` | K8s + Gemini AI CLI for assisted coding |

---

## 🎮 Usage Guide

### 1. Start & Enter
Start the container and enter the shell:
```bash
tazpod up
tazpod ssh
```

### 2. Using Base Mode (No Secrets)
If you just need the IDE tools, you can use the `base` image. Your project files in `/workspace` are always accessible.

### 3. Using Infisical & Secrets
To access your secrets securely:
1.  **Unlock**: Run `tazpod pull secrets`. It will ask for your LUKS passphrase and perform a sync.
2.  **Login**: If it's your first time, it will trigger `tazpod login`. The session token will be saved **inside the encrypted vault**.
3.  **Environment**: Run `tazpod env` to refresh environment variables in your current shell session.

### 4. Secrets Mapping (`.tazpod/secrets-sync-config.yml`)
Define which secrets to pull from Infisical and where to save them:

```yaml
config:
  infisical_project_id: "049af2e5-..." # Your project ID

secrets:
  - name: KUBECONFIG_CONTENT # Secret name in Infisical
    file: kubeconfig         # Target filename in ~/secrets/
    env: KUBECONFIG          # Exported environment variable
```

### 5. VPN & Remote Connectivity
Connect to your private lab or cloud providers securely:
```bash
tazpod vpn up   # Bring up the Wireguard tunnel
tazpod vpn down # Bring down the tunnel
```

### 6. Semantic Memory (Mnemosyne)
Ingest your technical history into a vector database:
```bash
tazpod memory sync /workspace/chats/md/ # Bulk ingestion
tazpod memory next /workspace/chats/md/ # Incremental ingest
tazpod memory wipe                      # Clear database
```

*   **Nomadic Identity**: Synchronize your entire environment (Vault, Config, AI Memories) via S3 for seamless portability.

---

## 🏗️ Technical Architecture

For a deep dive into the security model, Linux Namespaces, and the Go CLI internal logic, see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md), [docs/08-NOMADIC-IDENTITY.md](docs/08-NOMADIC-IDENTITY.md) and [docs/09-MEMORY-ENGINE.md](docs/09-MEMORY-ENGINE.md).

---
*Built with ❤️ by TazLab*
