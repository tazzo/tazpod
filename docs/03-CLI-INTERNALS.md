# TazPod CLI Internals (Go) ⚙️

TazPod v0.3.x is powered by a high-performance Go engine. It focuses on cryptographic integrity and efficient memory management.

## 1. Cryptographic Engine

TazPod uses a custom crypto implementation (`internal/crypto`) to handle the vault lifecycle without external dependencies like `cryptsetup`.

*   **Encryption**: AES-256 in **GCM (Galois/Counter Mode)**. GCM provides authenticated encryption, ensuring that the vault has not been tampered with.
*   **Key Derivation**: **PBKDF2** with SHA-256 and 100,000 iterations. This makes the vault highly resistant to brute-force attacks.
*   **Format**: The vault is a compressed Gzip TAR archive, encrypted and stored as `vault.tar.aes`.

---

## 2. RAM Enclave Orchestration

The core of the v0.3.x architecture is the **tmpfs** mount.

1.  **Mounting**: The CLI executes `mount -t tmpfs` to create a 64MB memory disk at `/home/tazpod/secrets`.
2.  **Extraction**: The decrypted TAR archive is extracted directly into this memory disk.
3.  **Permissions**: Files are extracted with `0600` permissions and owned by the `tazpod` user.

---

## 3. AWS Enclave Bridge

TazPod uses bind mounts to securely bridge secrets from the RAM enclave to standard locations expected by tools.

*   **AWS CLI**: `/home/tazpod/secrets/.aws` → `/home/tazpod/.aws`

This allows `aws` commands to work seamlessly while keeping credentials strictly in RAM.

---
*Next: Learn about the Ghost Mode in [04-GHOST-MODE.md](./04-GHOST-MODE.md)*
