# TazPod CLI Internals (Go) ⚙️

TazPod v0.2.0 is powered by a high-performance Go engine. It focuses on cryptographic integrity and efficient memory management.

## 1. Cryptographic Engine

TazPod uses a custom crypto implementation (`internal/crypto`) to handle the vault lifecycle without external dependencies like `cryptsetup`.

*   **Encryption**: AES-256 in **GCM (Galois/Counter Mode)**. GCM provides authenticated encryption, ensuring that the vault has not been tampered with.
*   **Key Derivation**: **PBKDF2** with SHA-256 and 100,000 iterations. This makes the vault highly resistant to brute-force attacks.
*   **Format**: The vault is a compressed Gzip TAR archive, encrypted and stored as `vault.tar.aes`.

---

## 2. RAM Enclave Orchestration

The core of the v0.2.0 architecture is the **tmpfs** mount.

1.  **Mounting**: The CLI executes `mount -t tmpfs` to create a 64MB memory disk at `/home/tazpod/secrets`.
2.  **Extraction**: The decrypted TAR archive is extracted directly into this memory disk.
3.  **Permissions**: Files are extracted with `0600` permissions and owned by the `tazpod` user.

---

## 3. Password Caching Strategy

To avoid redundant password prompts during a session (e.g., when doing `pull` which involves an unlock and multiple saves), TazPod implements a **Volatile Cache**:

*   When the vault is first unlocked, the passphrase is saved to `/home/tazpod/secrets/.vault_pass`.
*   Since this file resides in **RAM**, it is never written to physical disk.
*   CLI sub-processes read this file to perform silent cryptographic operations.
*   The cache is destroyed immediately when `tazpod lock` is executed.

---

## 4. Bridge & Bind Mechanics

TazPod uses **Bind Mounts** instead of symlinks for critical session paths (like Infisical).

```go
func bridge(local, vault string) {
    exec.Command("sudo", "mount", "--bind", vault, local).Run()
}
```

*   **Why?** Tools like Infisical often perform directory checks that fail on symlinks. Bind mounts are indistinguishable from regular directories to the application, providing 100% compatibility while keeping the data in RAM.

---

## 5. Signal Handling & Session Teardown

TazPod implements a robust cleanup hook in the `enter` command:

1.  User starts shell via `tazpod enter`.
2.  Go CLI waits for the Bash process to terminate.
3.  Upon termination, Go executes `tazpod lock`.
4.  `lock` performs a `lazy unmount` (`umount -l`) of all RAM-based paths, ensuring no sensitive data remains accessible in the container.

---
*Next: Learn about the secure memory isolation in [04-GHOST-MODE.md](./04-GHOST-MODE.md)*