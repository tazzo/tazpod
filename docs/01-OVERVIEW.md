# TazPod: The Zero Trust Development Enclave 🛡️

## 1. Introduction & Philosophy

TazPod is not just another container wrapper. It is a philosophy of development born from the need to reconcile **extreme security** with **developer convenience**.

In a modern DevOps environment, we handle powerful credentials (Kubeconfigs, Cloud API Keys, SSH Certificates). Storing them in plaintext on a laptop drive—even if encrypted at rest—exposes them to any process running with user privileges.

**TazPod's Core Mandate:**
> *Secrets must never touch the disk in plaintext. They exist only in RAM, inside an isolated kernel namespace, and vanish when the session ends.*

---

## 2. High-Level Architecture

TazPod operates by orchestrating three main components:

1.  **The CLI (Go)**: A static binary that manages the lifecycle, handles privileges, and interfaces with the Docker daemon.
2.  **The Vault (LUKS)**: An encrypted loopback file (`vault.img`) that acts as a secure portable drive for your secrets and session tokens.
3.  **The Ghost Shell (Namespaces)**: A Bash session running inside a private Mount Namespace. Inside this shell, secrets are visible. Outside, they don't exist.

---

## 3. Use Cases

### 🛠️ The Local Developer
*   **Scenario**: You are working on a Node.js app that needs AWS credentials.
*   **TazPod Solution**: Run `tazpod init` in your repo. The container mounts your code, but your AWS keys are pulled from Infisical directly into the RAM-gated vault. You code in Neovim/Tmux with full access, but a malware scanning your home directory sees nothing.

### ☸️ The Cluster Admin
*   **Scenario**: You manage multiple Kubernetes clusters (Prod, Staging, Dev).
*   **TazPod Solution**: Use `tazpod init k8s`. The image comes pre-loaded with `kubectl`, `helm`, `k9s`. Your Kubeconfig is securely pulled from Infisical only when you type your passphrase. No risk of accidentally running a command on Prod because the credentials aren't lying around.

### 🧠 The AI-Augmented Engineer
*   **Scenario**: You use Gemini or GPT tools that require API keys.
*   **TazPod Solution**: Use `tazpod init gemini`. The container includes the latest AI CLIs. Your API usage history and "memories" are persisted in a dedicated, isolated volume (`.gemini/`), keeping your context intact but segregated per project.

---

## 4. Key Differentiators

| Feature | Standard Dev Container | TazPod |
| :--- | :--- | :--- |
| **Storage** | Plaintext Volumes | LUKS Encrypted Image |
| **Secrets** | Env Vars / Files on Disk | RAM-Only Mounts |
| **Isolation** | Container Level | Kernel Namespace Level |
| **Persistence** | Permanent | Ephemeral (Ghost Mode) |
| **Toolchain** | Install on startup | Pre-baked Layered Images |

---
*Next: Learn how to set up your first TazPod in [02-INSTALL-INIT.md](./02-INSTALL-INIT.md)*
