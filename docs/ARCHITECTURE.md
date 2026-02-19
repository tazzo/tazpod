# TazPod Technical Architecture 🛡️🏗️

TazPod v0.2.0 is a specialized development enclave designed for **Zero-Trust workflows**. It implements a high-security, volatile execution environment using application-level cryptography and memory isolation.

---

## 1. Multi-Layer Architecture

TazPod operates across three isolated layers to minimize the attack surface:

1.  **Orchestration (Host)**: A Go-based CLI (`tazpod`) that manages the Docker lifecycle, project-specific unique container identifiers, and cryptographic operations.
2.  **Volatile Enclave (Memory)**: A **tmpfs** RAM disk mounted within the container. Decrypted secrets reside only here.
3.  **Application (Container)**: Optimized Docker images containing toolstacks (IDE, Infisical, K8s, AI).

---

## 2. Semantic Memory Layer 🧠

Beyond raw data, TazPod implements a persistent intelligence layer to capture and index technical insights.

### 2.1 Fact Extraction
Using the **Gemini 2.0 Flash** model via CLI, TazPod parses session logs to extract structured "technical chronicles". This process filters out noise and focuses on problem-solution pairs.

### 2.2 Vector Storage (pgvector)
Extracted facts are transformed into high-dimensional vectors (embeddings) and stored in a local PostgreSQL instance. This enables semantic search, allowing the environment to provide context-aware suggestions based on past experiences.

---

## 3. The RAM Boundary Model ☁️

The core security principle of v0.2.0 is the **RAM-Only Decryption** and **Project Isolation**.

### 2.1 Project Isolation
Every project initialized with `tazpod init` receives a unique `container_name` (e.g., `tazpod-<folder>-<rand>`). This allows developers to work on multiple projects simultaneously, each with its own isolated RAM enclave and toolset, without any resource collision.

### 2.2 Encryption at Rest
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
