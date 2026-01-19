# TazPod Architecture & Security Model 🛡️

TazPod is not just a development container. It is an ephemeral, **Zero Trust** infrastructure designed to ensure that secrets are never exposed in plaintext on the host filesystem or to unauthorized processes.

## 1. Core Concept: "Ghost Mode" 👻

The heart of TazPod's security is **Ghost Mode**. Unlike traditional Docker volumes (which are visible to the entire container), TazPod leverages **Linux Namespaces** to create an isolated "bubble".

### The Concurrency Problem
In a standard container, if you mount a decrypted volume at `/home/user/secrets`, anyone who gains access to the container (e.g., via another `docker exec` session or a compromised process) can read those files.

### The Solution: Namespace Isolation
When you run `tazpod unlock`, the system does not simply mount the disk. Instead:
1.  The Go binary executes `unshare -m --propagation private`.
2.  This creates a new **Mount Namespace** in the Linux kernel.
3.  Inside this namespace (and ONLY here), the encrypted disk is mounted.
4.  A new Bash Shell is launched (the "Ghost Shell").

**Result:**
*   **Inside the Ghost Shell:** You see the files in `~/secrets`.
*   **Outside (other shells, host):** The `~/secrets` directory appears **EMPTY**. Even `root` on the host cannot see the mountpoint because it exists only in the memory of the isolated process.

---

## 2. The "Matryoshka" Shell Lifecycle 🐚

TazPod's execution flow is a chain of nested processes designed to ensure secrets are destroyed as soon as the user stops working.

1.  **Host Shell (Mac/Linux)**
    *   Runs `tazpod ssh` -> Executes `docker exec -it tazpod-lab bash`.
2.  **Container Entry Shell (Bash)**
    *   This is a "public" shell. It has no access to secrets.
    *   The user runs `tazpod unlock`.
3.  **Go Wrapper (Parent)**
    *   Verifies the passphrase.
    *   Runs `unshare` to create the namespace.
4.  **Go Wrapper (Internal Ghost - Root)**
    *   Runs inside the namespace.
    *   Opens LUKS (`cryptsetup`).
    *   Mounts the ext4 filesystem.
    *   Performs *Drop Privileges* (switches back to user `tazpod`).
    *   Loads environment variables from Infisical.
5.  **Ghost Shell (Bash - User)**
    *   This is the shell where the user works. It has access to secrets.
6.  **Exit / Death**
    *   When the user types `exit` in the Ghost Shell:
        1.  The Ghost Shell dies.
        2.  `Internal Ghost` (Go) regains control.
        3.  Executes `umount`, `cryptsetup close`, `dmsetup remove`.
        4.  Secrets vanish from kernel memory.
        5.  The Go process terminates.
    *   The **Container Entry Shell** (step 2) has a helper function in `.bashrc` that detects the exit and forces a cascading `exit`, closing the SSH connection.

---

## 3. Storage & Persistence 💾

### The Vault (`vault.img`)
*   It is a simple allocated file (`dd`) residing on the host at `.tazpod-vault/vault.img`.
*   It is mapped into the container as a **Loop Device** (`/dev/loopX`).
*   It is encrypted with **LUKS2**. The key is never saved to disk; it resides only in kernel RAM during the Ghost session.

### Infisical Integration
*   TazPod uses the Infisical CLI to pull secrets.
*   **Session Persistence**: The Infisical login token (`~/.infisical`) is moved inside the encrypted vault (`~/secrets/.auth-infisical`) and symlinked.
*   This way, if the vault is closed, no one can steal your Infisical session.

---

## 4. Device Mapper Troubleshooting 🔧

Docker and Kernel Device Mappers sometimes conflict ("Device or resource busy"). TazPod uses a "Smart Cleanup" strategy:
1.  **Lazy Unmount (`umount -l`)**: Detaches the filesystem even if the shell is holding it open.
2.  **Kernel Check**: Queries `dmsetup info` instead of checking files in `/dev/mapper` (which are often missing in containers).
3.  **SIGKILL**: The `tazpod lock` command uses instant death signals to force the closure of shells holding the vault hostage.
