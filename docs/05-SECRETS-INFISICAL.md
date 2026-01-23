# Secrets Management with Infisical 🔐

TazPod is designed to be "Infisical-Native". We don't just copy `.env` files; we integrate the Infisical CLI directly into the secure lifecycle of the container.

## 1. The Persistence Challenge

Infisical requires authentication. When you run `infisical login`, it stores a token in `~/.infisical/`.
In a standard ephemeral container, this token is lost on restart.
If we mount `~/.infisical` from the host, the token is exposed in plaintext on your disk.

### The Solution: The Vault Bridge

TazPod moves the persistence layer **inside** the encrypted vault.

1.  **Storage**: The real data lives at `/home/tazpod/secrets/.infisical-vault`.
2.  **Bridging**: When you unlock the vault, TazPod executes a **Bind Mount**:
    ```bash
    mount --bind /home/tazpod/secrets/.infisical-vault /home/tazpod/.infisical
    ```
3.  **Result**: The Infisical CLI sees its config file where it expects it to be, but the data is physically located on the encrypted loop device.

> **Note**: Gemini AI (`.gemini/`) uses this same "Vault Bridge" strategy to ensure your conversation history and API keys never stay on the unprotected host disk.

---

## 2. Secrets Mapping (`secrets.yml`)

Instead of manual `export` commands, TazPod uses a declarative file in your project root: `secrets.yml`.

```yaml
config:
  infisical_project_id: "your-project-id"

secrets:
  - name: KUBE_CONFIG      # The secret name in Infisical Cloud
    file: kubeconfig       # Will be saved to ~/secrets/kubeconfig
    env: KUBECONFIG        # Will export KUBECONFIG=~/secrets/kubeconfig
```

This file is safe to commit to Git because it contains **no actual secrets**, only the map of where to find them.

---

## 3. The `tazpod pull` Workflow

When you run `tazpod pull` inside the container:

1.  **Unlock**: Checks if the vault is open. If not, prompts for passphrase.
2.  **Auth Check**: Verifies if a valid Infisical session exists in the bridge.
3.  **Login**: If session is invalid, triggers `infisical login` (interactive flow).
4.  **Sync**:
    *   Downloads generic environment variables to `~/secrets/.env-infisical`.
    *   Downloads specific files defined in `secrets.yml`.
    *   Sets strict permissions (`0600`) on all downloaded files.

---
*Next: Explore the container images in [06-LAYERS-IMAGES.md](./06-LAYERS-IMAGES.md)*
