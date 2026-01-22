# Installation & Initialization Guide 🚀

## 1. Global Installation

TazPod is distributed as a single static Go binary. We provide a universal installer script that handles OS detection (Linux/macOS) and architecture (AMD64/ARM64).

**One-Line Install:**
```bash
curl -sSL https://raw.githubusercontent.com/tazzo/tazpod/master/scripts/install.sh | bash
```

**What it does:**
1.  Downloads the latest binary from GitHub Releases.
2.  Installs it to `~/.local/bin/tazpod`.
3.  Sets executable permissions.
4.  Checks if `~/.local/bin` is in your `$PATH`.

---

## 2. Project Initialization (`tazpod init`)

TazPod is designed to be **per-project**. You don't just "run TazPod"; you initialize a directory to *be* a TazPod workspace.

### The Command
```bash
# Basic init (defaults to k8s image)
tazpod init

# Specialized init (choose your stack)
tazpod init base      # Just OS + IDE
tazpod init infisical # OS + IDE + Secrets Manager
tazpod init k8s       # OS + IDE + Secrets + DevOps Tools
tazpod init gemini    # The Full Package (AI + K8s)
```

### What happens during `init`?
The CLI performs the following actions:
1.  **Creates `.tazpod/`**: A hidden directory for project-local data.
2.  **Generates `config.yaml`**: The blueprint for your container.
3.  **Creates `secrets.yml`**: A template for mapping Infisical secrets.
4.  **Creates `Dockerfile`**: A sample file in `.tazpod/` to let you customize the image.
5.  **Secures `.gitignore`**: Automatically ignores the `vault/` and `.gemini/` directories to prevent accidental commits of sensitive data.

---

## 3. Anatomy of `.tazpod/`

This folder is the brain of your environment.

```text
/my-project/
├── .tazpod/
│   ├── config.yaml       # Container configuration
│   ├── Dockerfile        # Custom build instructions (optional)
│   ├── .gitignore        # Ignores vault and memory
│   └── vault/            
│       └── vault.img     # (Created after first use) The Encrypted LUKS Container
```

### The `config.yaml`
This file tells the CLI how to behave.

```yaml
version: 1.0
image: "tazzo/tazlab.net:tazpod-gemini" # The Docker image to pull/build
container_name: "tazpod-myproject-839201" # Unique ID generated during init
user: "tazpod" # The non-root user inside the container
features:
  ghost_mode: true # Enables the Namespace Isolation logic
  debug: false     # Set to true for verbose CLI logs
```

---

## 4. First Start (`tazpod up`)

Once initialized, you start the daemon:

```bash
tazpod up
```

**What happens:**
1.  **Build/Pull**: If you customized the Dockerfile, it builds it. Otherwise, it pulls the image from Docker Hub.
2.  **Mounts**:
    *   Mounts the current directory (`$PWD`) to `/workspace`.
    *   Mounts the local `.gemini/` folder to `/home/tazpod/.gemini` (for AI persistence).
3.  **Run**: Starts the container in detached mode (`-d`) with `sleep infinity`. It waits for you.

---
*Next: Dive into the engine in [03-CLI-INTERNALS.md](./03-CLI-INTERNALS.md)*
