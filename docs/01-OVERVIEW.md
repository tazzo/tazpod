# TazPod: The Zero Trust Development Enclave 🛡️

## 1. Introduction & Philosophy

TazPod v0.2.0 is a refined evolution of the zero-trust development environment. It reconciles **extreme security** with **developer convenience** by moving away from kernel-level complexity (LUKS/Namespaces) towards a more portable and performant **RAM-based architecture**.

In modern DevOps, we handle critical credentials (Kubeconfigs, Cloud API Keys). TazPod ensures these secrets are protected at rest and volatile during use.

**TazPod's Core Mandate:**
> *Secrets must never stay on disk in plaintext. They are decrypted into a RAM disk (tmpfs), bridged to their application paths, and vanish instantly when the session ends or the vault is locked.*

---

## 2. High-Level Architecture (v0.2.0)

TazPod orchestrates three main layers:

1.  **The CLI (Go)**: A high-performance binary that manages the container lifecycle, RAM disk orchestration, and AES-256-GCM cryptographic operations.
2.  **The Encrypted Vault (AES-GCM)**: A single portable file (`vault.tar.aes`) containing your secrets and session tokens, protected by AES-256-GCM encryption with PBKDF2 key derivation.
3.  **The RAM Enclave (tmpfs)**: A secure memory space (`/home/tazpod/secrets`) where data is decrypted and extracted. Applications see their config files via **Bind Mounts**, unaware that the data resides in volatile memory.

---

## 3. Use Cases

### 🛠️ The Local Developer
*   **Scenario**: You need AWS credentials for a project but don't want them in your home directory.
*   **TazPod Solution**: Run `tazpod pull`. It mounts the RAM enclave, pulls keys from Infisical, and saves the updated encrypted vault. When you `exit`, the RAM is wiped.

### ☸️ The Cluster Admin
*   **Scenario**: Managing multiple sensitive Kubernetes clusters.
*   **TazPod Solution**: Use `tazpod init k8s`. Your Kubeconfigs are stored in the vault. They are only available after providing your master passphrase, preventing unauthorized cluster access.

### 🧠 The AI-Augmented Engineer
*   **Scenario**: You use Gemini or LLM tools that require persistent memories and API tokens.
*   **TazPod Solution**: Use `tazpod init gemini`. Gemini's configuration is kept in the vault (RAM), while non-sensitive logs are persisted to the host workspace for auditability.

---

## 4. Key Differentiators (v0.2.0 vs v0.1.x)

| Feature | TazPod v0.1.x (Legacy) | TazPod v0.2.0 (Stable) |
| :--- | :--- | :--- |
| **Encryption** | LUKS2 (Disk Image) | AES-256-GCM (File-based) |
| **Storage** | 512MB Loopback File | Dynamic TAR Archive |
| **Isolation** | Kernel Namespaces | tmpfs RAM Disk |
| **Portability** | Requires `cryptsetup` | Zero dependencies (Pure Go) |
| **Persistence** | Permanent Mounts | Auto-cleanup on Lock/Exit |

---
*Next: Learn how to set up your environment in [02-INSTALL-INIT.md](./02-INSTALL-INIT.md)*