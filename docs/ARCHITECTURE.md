# TazPod Technical Architecture 🛡️🏗️

TazPod is a specialized, ephemeral development environment designed for **Zero-Trust workflows**. It combines modern containerization with kernel-level security features to ensure that sensitive credentials remain strictly isolated and non-persistent.

---

## 1. High-Level Architecture

TazPod operates across three distinct layers:

1.  **Orchestration Layer (Host)**: A Go-based CLI (`tazpod`) that manages the container lifecycle, project initialization, and secure entry points.
2.  **Enclave Layer (Kernel)**: Uses **Linux Mount Namespaces** and **LUKS2 encryption** to create a "Ghost Mode"—a secure memory space invisible to the host and other container processes.
3.  **Application Layer (Container)**: Modular Docker images (Verticals) providing tailored toolstacks (IDE, Infisical, Kubernetes, AI).

---

## 2. The "Ghost Mode" Security Model 👻

The core innovation of TazPod is the **Ghost Mode**. In standard Docker setups, any process inside a container can see all mounted volumes. Ghost Mode breaks this paradigm.

### 2.1 Namespace Isolation
When `tazpod unlock` or `tazpod pull` is executed:
*   The Go binary invokes the `unshare` system call with the `--mount` and `--propagation private` flags.
*   This spawns a **new Mount Namespace** for that specific process tree.
*   The encrypted vault is mounted **only within this namespace**.

**Security Impact:** Any concurrent `docker exec` session or compromised process running in the "main" container space will see an **empty** `~/secrets` directory. The decrypted files exist only in the kernel memory context of the Ghost session.

### 2.2 LUKS2 Encryption
The data resides in a loopback image file (`vault.img`) located at `.tazpod/vault/`. 
*   **Encryption**: AES-XTS 256-bit (Standard LUKS2).
*   **Decryption**: Performed via `cryptsetup` inside the container.
*   **Zero-Persistence**: The decryption key exists only in the RAM of the isolated Ghost process.

---

## 3. Persistent Identity & Infisical Enclave 🔐

Infisical's session tokens are sensitive. Storing them in the standard home directory within a container is insecure. 

### 3.1 Unified Vault Persistence
TazPod standardizes identity storage in `~/secrets/.infisical-vault`. 
*   **Bridging**: TazPod uses a **Bind Mount** to bridge the standard config path (`~/.infisical`) and the keyring path (`~/infisical-keyring`) directly into the encrypted vault.
*   **Ownership Management**: The CLI performs recursive `chown` operations to ensure the non-root `tazpod` user (UID 1000) maintains full access to the enclave while the root wrapper performs system-level mounts.

---

## 4. The Shell Matryoshka (Process Lifecycle) 🐚

TazPod manages a complex chain of shell executions to ensure a seamless developer experience:

1.  **Terminal Entry**: `tazpod ssh` initiates a `docker exec` into a public Bash shell.
2.  **The Unlock Trigger**: The user runs `tazpod pull`.
3.  **Privilege Escalation & Isolation**: The Go CLI uses `sudo unshare` to jump into the Enclave context.
4.  **Hardware Unlock**: LUKS is opened, the filesystem is mounted, and the Infisical bridge is established.
5.  **Privilege Drop**: The CLI drops root privileges and spawns a **Ghost Bash Shell** as the `tazpod` user.
6.  **Cleanup on Exit**: Once the Ghost Shell terminates, the Go wrapper intercepts the signal, performs a `lazy unmount` (`umount -l`), closes the LUKS mapper, and destroys the namespace.

---

## 5. Modular Image Hierarchy (Verticals) 🧅

TazPod uses a layered image strategy to minimize build times and maximize portability:

1.  **`tazpod-base`**: Ubuntu 24.04 + IDE (Neovim, Zellij, Starship, Lazygit).
2.  **`tazpod-infisical`**: Adds Infisical CLI and the secret injection engine.
3.  **`tazpod-k8s`**: Adds the full DevOps stack (Kubectl, Helm, K9s, Talosctl).
4.  **`tazpod-gemini`**: Adds the Gemini AI CLI for integrated platform mentoring.

---

## 6. Smart CLI Workflow 🧠

The `tazpod` Go binary implements an "Intent-Based" workflow:
*   **`init`**: Bootstraps a project with `config.yaml`, `Dockerfile` templates, and `secrets.yml`.
*   **`up`**: Orchestrates `docker build` (if custom layers exist) and starts the container.
*   **`pull`**: A unified command that checks for vault state, sifts through legacy sessions, authenticates with Infisical, and synchronizes secrets in one go.
*   **`env`**: A secure bridge that refreshes shell variables via `eval $(tazpod env)` without ever printing secrets to the TTY.

---
*Architecture v9.3 | Documented by Senior Platform Mentor*