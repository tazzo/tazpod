# TazPod: The Zero-Trust Containerized Developer Environment 🛡️📦

TazPod is an ephemeral, secure, and portable development environment built on **Docker**, **Go**, and **Gopass + GPG**. It provides a fully configured IDE (Neovim, Tmux) while keeping sensitive credentials encrypted on disk and decrypted only in GPG agent memory with strict TTLs, eliminating plaintext credentials from the host filesystem.

---

## 🚀 Key Features

*   **Gopass Secrets Store**: Centralized secret management inside the container, backed by a secure git repository (default: `/workspace/tazlab-secrets`).
*   **GPG Passphrase Caching**: Decryption of GPG keys in memory via GPG Agent. Cache is set to expire after 1 hour of inactivity (resets on active use), with a 7-day hard maximum.
*   **Safe TTY Binding**: Native TTY alignment using `gpg-connect-agent updatestartuptty /bye` to support non-interactive pinentry in TMUX panes and parallel shells.
*   **Reduced Privileges**: Container runs without elevated host privileges (removed `--cap-add SYS_ADMIN`), keeping only `NET_ADMIN` for Tailscale networking.
*   **Transparent Orchestration**: Works identically on local Docker containers and Proxmox LXC CT nodes (LXC mode).

---

## 📥 Installation

Install TazPod globally on your system:

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
*   `.tazpod/config.yaml`: The main configuration file (deployment mode, image name, container name, gopass store path).
*   `.tazpod/` agent folders: `.pi`, `.omp`, `.gemini`, `.claude`, `.aws`, `.opencode`, `.herdr` for persistent configuration tracking.

---

## ⚙️ Configuration (`config.yaml`)

The `.tazpod/config.yaml` file defines your environment's blueprint:

```yaml
mode: docker
image: tazzo/tazpod-ai:latest
container_name: tazpod-lab
user: tazpod
ghost_mode: true
features:
    debug: false
gopass:
    store: /workspace/tazlab-secrets
providers: {}
```

---

## ☁️ Pre-compiled Images (Verticals)

| Image Name | Features |
| :--- | :--- |
| `tazzo/tazpod-base` | Ubuntu 24.04, Neovim, Tmux, Shell tools (Bash/Starship), GPG, Gopass |
| `tazzo/tazpod-aws` | Base + AWS CLI v2 |
| `tazzo/tazpod-k8s` | AWS + Kubectl, Helm, K9s, Talosctl, Stern, Terraform |
| `tazzo/tazpod-ai` | K8s + Gemini AI CLI, Pi Agent, Mnemosyne |

---

## 🎮 Usage Guide

### 1. Smart Entry (No Arguments or `enter`)

Run either `tazpod` or `tazpod enter` for the guided smart bootstrap flow:
1.  **Init**: Setup project directories if missing.
2.  **Up**: Ensure the container is running (creates/starts if needed).
3.  **Enter**: Open the shell in the container.

### 2. CLI Commands

| Command | Action |
| :--- | :--- |
| `tazpod init` | Initialize a new TazPod workspace |
| `tazpod up` | Create and start the development container |
| `tazpod down` | Stop and remove the development container |
| `tazpod enter` | Open the shell in the container |
| `tazpod update` | Pull the latest version of the configured image |
| `tazpod gopass` | Interactive setup to import keys and mount the gopass store |
| `tazpod lock` | Revoke cached GPG keys and close the store cache |
