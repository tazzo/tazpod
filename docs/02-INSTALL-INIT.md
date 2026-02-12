# Installation & Initialization Guide 🚀

## 1. Global Installation

TazPod is distributed as a single static Go binary. The universal installer handles OS detection and places the binary in your local path.

**One-Line Install:**
```bash
curl -sSL https://raw.githubusercontent.com/tazzo/tazpod/master/scripts/install.sh | bash
```

**System Requirements:**
*   **Docker**: Must be installed and running.
*   **Permissions**: Your user must be in the `docker` group or have `sudo` access.

---

## 2. Project Initialization (`tazpod init`)

TazPod is project-centric. You initialize a directory to transform it into a secure workspace.

### The Command
```bash
# Initialize with the default Gemini image
tazpod init
```

### What happens during `init`?
The CLI performs the following actions:
1.  **Creates `.tazpod/`**: A project-local metadata directory.
2.  **Generates `config.yaml`**: Defines image, user, and a **unique container name** based on the current folder and a random suffix (e.g., `tazpod-myproject-4829`). This ensures that multiple TazPod projects can run concurrently without naming conflicts.
3.  **Creates `.tazpod/secrets-sync-config.yml`**: A template for Infisical secret mapping.
4.  **Secures `.gitignore`**: Prevents accidental commits of `vault/` and `.gemini/` local data.

---

## 3. Anatomy of `.tazpod/`

```text
/my-project/
├── .tazpod/
│   ├── config.yaml                # Container blueprint
│   ├── secrets-sync-config.yml     # Secrets mapping (Safe for Git)
│   └── vault/            
│       └── vault.tar.aes          # The Encrypted Secrets Storage
```

### The `config.yaml`
```yaml
version: 1.0
image: "tazzo/tazlab.net:tazpod-gemini"
container_name: "tazpod-myproject-4829"
user: "tazpod"
```

---

## 4. Lifecycle Commands

### Starting the Pod (`tazpod up`)
Starts the Docker container in the background. It dynamically mounts your current directory to `/workspace`.

### Entering the Shell (`tazpod enter`)
Enters the container interactivelly. 
*   **Auto-Cleanup**: When you type `exit`, TazPod automatically triggers a `lock` to unmount and secure the RAM enclave.

### Sinking the Pod (`tazpod down`)
Stops and removes the container.

---
*Next: Dive into the engine in [03-CLI-INTERNALS.md](./03-CLI-INTERNALS.md)*