# TazPod CLI Internals (Go) ⚙️

TazPod is written in **Go** to leverage strong typing, single-binary distribution, and direct access to Linux syscalls. This document explains the internal logic of the `cmd/tazpod/main.go` source.

## 1. Command Architecture

The CLI follows a standard "subcommand" pattern using a main switch statement.

```go
func main() {
    // ... args parsing ...
    switch arg {
    case "up": up()
    case "down": down()
    case "enter", "ssh": enter()
    case "pull", "sync": pull()
    case "login": login()
    case "internal-ghost": internalGhost()
    // ...
    }
}
```

### Why not a library like Cobra?
For TazPod, we chose **zero dependencies** for the CLI structure to keep the binary small and the logic transparent. Every command maps directly to a function that orchestrates Docker or OS calls.

---

## 2. Privilege Management

One of the most complex aspects of TazPod is managing the dance between the user (`tazpod` inside container, or your user on host) and `root`.

*   **`tazpod up`**: Runs as **User**. Calls `docker run`. The user must have permission to talk to the Docker daemon.
*   **`tazpod unlock` / `pull`**: Runs as **User**, but executes `sudo unshare`.
    *   This is critical. We need `root` privileges to mount the loop device and create the namespace, but we immediately drop back to the user context inside the Ghost Shell.

### The "Sudo" Wrapper
When you run `tazpod pull` inside the container, the binary detects it needs elevation:

```go
cmd := exec.Command("sudo", "unshare", "--mount", "--propagation", "private", "/usr/local/bin/tazpod", "internal-ghost", "pull")
```

This re-executes the binary with a special hidden command (`internal-ghost`).

---

## 3. The `internal-ghost` Function

This is where the magic happens. This function is **only** ever executed by `root` inside a fresh namespace.

1.  **Unlock**: Prompts for passphrase and calls `cryptsetup open`.
2.  **Mount**: Mounts the decrypted mapper device to `/home/tazpod/secrets`.
3.  **Migration**: Checks for legacy data structures and migrates them.
4.  **Bridge**: Sets up the bind-mounts for `.infisical` and `infisical-keyring`.
5.  **Ownership Fix**: Runs `chown -R` to ensure the user can read what root just mounted.
6.  **Handover**: Spawns a `bash` shell, dropping privileges back to `UID 1000`.

---

## 4. Signal Handling & Cleanup

TazPod must be a good citizen. Leaving encrypted volumes open is a security risk.

*   The Go process waits for the child `bash` shell to exit.
*   Upon exit, it triggers `cleanupMappers()`:
    1.  `umount -l` (Lazy unmount) of the secrets directory.
    2.  `cryptsetup close` to wipe the key from kernel memory.
    3.  `dmsetup remove` to delete the device mapper node.

This ensures that once the shell closes, the data is cryptographically inaccessible again.

---
*Next: Understand the isolation mechanism in [04-GHOST-MODE.md](./04-GHOST-MODE.md)*
