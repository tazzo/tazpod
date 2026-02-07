# TazPod Technical Architecture 🛡️🏗️

TazPod v0.2.0 is a specialized development enclave designed for **Zero-Trust workflows**. It implements a high-security, volatile execution environment using application-level cryptography and memory isolation.

---

## 1. Multi-Layer Architecture

TazPod operates across three isolated layers to minimize the attack surface:

1.  **Orchestration (Host)**: A Go-based CLI (`tazpod`) that manages the Docker lifecycle and cryptographic operations.
2.  **Volatile Enclave (Memory)**: A **tmpfs** RAM disk mounted within the container. Decrypted secrets reside only here.
3.  **Application (Container)**: Optimized Docker images containing toolstacks (IDE, Infisical, K8s, AI).

---

## 2. The RAM Boundary Model ☁️

The core security principle of v0.2.0 is the **RAM-Only Decryption**.

### 2.1 Encryption at Rest
Secrets are stored in a compressed TAR archive (`vault.tar.aes`).
*   **Algorithm**: AES-256-GCM (Authenticated Encryption).
*   **Derivation**: PBKDF2 (100,000 iterations).
*   **Salt**: Randomly generated per encryption.

### 2.2 Volatile Execution
When the vault is unlocked:
1.  A **tmpfs** is mounted at `/home/tazpod/secrets`.
2.  The TAR archive is extracted directly into the tmpfs.
3.  **Zero Leakage**: No unencrypted data ever touches the persistent storage of the container or the host.

---

## 3. Auth Persistence (The Bridge) 🔗

To ensure a seamless experience with Infisical and Gemini without sacrificing security, TazPod uses a **Bridging Logic**:

*   **Enclave Targets**: Persistent config paths (e.g., `~/.infisical`) are **Bind-Mounted** to the RAM Enclave.
*   **Stateless Tooling**: Applications treat these paths as regular directories, unaware that their session tokens are actually in RAM.
*   **Auto-Save Trigger**: Any command that modifies secrets (e.g., `tazpod pull`) triggers a re-encryption of the RAM Enclave to persist the state back to the host disk.

---

## 4. The Smart Environment Bridge 🧹

TazPod maintains a "Clean Environment" policy.

1.  **`eval $(tazpod env)`**: When the vault is open, this exports secret paths to the shell.
2.  **`tazpod lock`**: Automatically triggers `unset` commands for all enclave variables.
3.  **Timing**: A 100ms grace period is implemented in the shell function to ensure kernel mount updates are reflected in the environment.

---

## 5. Security Lifecycle

| Phase | Action | Result |
| :--- | :--- | :--- |
| **Unlocked** | AES Decrypt -> RAM Mount | Secrets available in TTY |
| **Active** | Bind Mount Auth Paths | Tools authenticated |
| **Locked** | umount -l -> Wipe RAM | Secrets cryptographically gone |
| **Exit** | Auto-Lock Hook | Enclave secured on session end |

---
*Architecture v0.2.0 | Documented by Senior Platform Mentor*
