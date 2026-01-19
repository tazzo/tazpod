# TazPod: The Zero-Trust Containerized Developer Environment 🛡️📦

TazPod is an ephemeral, secure, and portable development environment built on **Docker**, **Go**, and **Linux Namespaces**. It provides a fully configured IDE (Neovim, Tmux, Zellij) while ensuring that sensitive secrets are never exposed to the host filesystem or unauthorized processes.

## 🚀 Key Features

*   **Zero Trust Architecture**: Secrets are stored in a LUKS-encrypted vault (`vault.img`) that is mounted only within an isolated **Linux Namespace** ("Ghost Mode"). Even `root` on the host cannot see your secrets.
*   **Infrastructure as Code**: Defined via `Dockerfile` and `config.yaml`.
*   **Full IDE Stack**: Ubuntu 24.04, Neovim (v0.10+), LazyVim, NVM (Node LTS), Lazygit, Yazi, Zellij, Tmux.
*   **Modern Shell**: Bash with Starship (Pastel preset), Zoxide, Eza, Bat, Ripgrep, FZF.
*   **Infisical Integration**: Securely pull secrets from Infisical with persisted authentication sessions inside the vault.
*   **Portable**: Runs on any machine with Docker (Linux/macOS).

## 📥 Installation

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/tazzo/tazpod.git
    cd tazpod
    ```

2.  **Build and Start:**
    ```bash
    ./tazpod up
    ```

## 🎮 Usage

### 1. Enter the Pod
Access the container shell from your host:
```bash
./tazpod ssh
```

### 2. Unlock the Vault (Ghost Mode)
Inside the container, unlock your secure vault. This creates a private namespace and mounts your secrets at `~/secrets`.
```bash
tazpod unlock
```
*If this is your first time, it will ask you to define a Master Passphrase.*

### 3. Manage Secrets
Once inside the Ghost Shell:
*   **Login to Infisical**: `tazpod login`
*   **Sync Secrets**: `tazpod pull` (Updates `~/secrets/.env-infisical` and files defined in `secrets.yml`)

### 4. Lock & Exit
*   **Exit**: Simply type `exit`. This will automatically unmount the vault, close LUKS, destroy the namespace, and close your SSH session.
*   **Lock (Stay inside)**: Type `tazpod lock`. This forces the vault to close but keeps you in the container (useful for maintenance).

### 5. Re-initialize
To wipe the vault and start fresh:
```bash
tazpod reinit
```
*(Note: You must NOT be in Ghost Mode to run this).*

## 🏗️ Architecture

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for a deep dive into the security model, Linux Namespaces, and the Go CLI internal logic.

## 🛠️ Tech Stack

*   **Core**: Go 1.23, Docker
*   **OS**: Ubuntu 24.04 LTS (Noble)
*   **Editor**: Neovim + LazyVim
*   **Terminal**: Tmux, Zellij, Starship
*   **Tools**: Lazygit, Yazi, Eza, Bat, Ripgrep, FZF
*   **Security**: Cryptsetup (LUKS2), Infisical CLI

---
*Built with ❤️ by TazLab*
