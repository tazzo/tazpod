# The Ghost Mode: Linux Namespaces Explained 👻

Ghost Mode is the security feature that sets TazPod apart from standard dev containers. It solves the problem of **Concurrent Access**: how to let *you* see the secrets, but prevent *anyone else* on the same machine from seeing them.

## 1. The Linux Mount Namespace

In Linux, the "filesystem tree" is not a global singleton. It is a property of a **Namespace**.
By default, all processes live in the Global Namespace. If you mount a disk at `/mnt/data`, everyone sees it.

TazPod uses the `unshare(CLONE_NEWNS)` syscall.
This creates a **copy** of the current mount tree for the new process. Changes made inside this new tree (like mounting a decrypted volume) **do not propagate** back to the parent or to other namespaces.

### Visualizing the Isolation

*   **Process A (Docker Daemon)**: Sees `/home/tazpod/secrets` as an empty directory.
*   **Process B (Intruder)**: `docker exec ls -la /home/tazpod/secrets` -> Empty.
*   **Process C (You / Ghost)**: `ls -la /home/tazpod/secrets` -> **Full Access**.

The decrypted data exists literally nowhere else but in the memory context of your specific shell session.

---

## 2. The Matryoshka Shell Lifecycle 🪆

TazPod implements a nested shell strategy to manage this isolation safely.

1.  **Outer Shell**: The entry point. You are `tazpod`. No secrets.
2.  **Sudo/Unshare**: You request entry. The system elevates to `root` and forks a new namespace.
3.  **Setup Phase**: The `internal-ghost` (as root) prepares the room. It decrypts LUKS, mounts drives, sets permissions.
4.  **Inner Shell (Ghost)**: The system drops privileges and gives you a `bash` prompt. You are `tazpod` again, but now the room is furnished with secrets.
5.  **Teardown**: When you type `exit`, the Inner Shell dies. The Setup Phase resumes, wipes the room (unmount/close), and then the process dies, returning you to the Outer Shell.

---

## 3. The `.bashrc` Integration

To make this seamless, TazPod injects smart functions into the container's `.bashrc`.

**The Core Wrapper:**
```bash
tazpod() {
    # Special case for 'env' to prevent leaking secrets to TTY
    if [ "$1" == "env" ]; then
        eval "$(/usr/local/bin/tazpod __internal_env 2>/dev/null)"
        return 0
    fi
    /usr/local/bin/tazpod "$@";
}
```

**The Gemini Safety Latch:**
For AI tools, we add a wrapper that prevents execution outside the vault.
```bash
gemini() {
    if [ "$TAZPOD_GHOST_MODE" = "true" ]; then
        /usr/local/bin/gemini "$@"
    else
        echo "🔒 Vault is closed. Unlocking required..."
        tazpod unlock
    fi
}
```

## 4. Cross-Platform Compatibility (macOS) 🍏

A common question is: "How can Linux Namespaces and LUKS work on macOS?"

The answer lies in the **Docker Engine architecture**. On macOS, Docker Desktop or OrbStack run a lightweight Linux VM. When you execute `tazpod` inside the container:
1.  The syscalls (`unshare`, `mount`) are handled by the **Linux kernel of the VM**, not the host's Darwin kernel.
2.  The `--privileged` flag allows the container to interact with the VM's Device Mapper.

This means TazPod is architected to provide the **same level of security and isolation on a Mac** as it does on a native Linux machine. 

*Disclaimer: Current validation tests have been performed primarily on Linux environments. While the underlying Docker VM technology on macOS supports these kernel features, platform-specific edge cases may exist.*

---
*Next: Learn how we manage secrets in [05-SECRETS-INFISICAL.md](./05-SECRETS-INFISICAL.md)*
