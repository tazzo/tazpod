# Secrets Management with Infisical 🔐

TazPod is built to be the ideal companion for **Infisical**. It handles the entire lifecycle of pulling, injecting, and persisting secrets without ever exposing them to the host filesystem in plaintext.

## 1. The Persistence Loop

Infisical tokens are volatile by design. TazPod ensures they are persisted **only in their encrypted state**.

1.  **Unlock**: Mounts the RAM Enclave.
2.  **Login**: User runs `tazpod login`. The session token is written to `~/.infisical` (which is bind-mounted to RAM).
3.  **Save**: TazPod automatically executes `Save()`, encrypting the RAM Enclave (including the session token) back to `vault.tar.aes`.
4.  **Persistent Session**: Next time you `unlock`, the session token is restored to RAM, and you are automatically authenticated.

---

## 2. Declarative Mapping (`secrets.yml`)

TazPod uses `secrets.yml` to define which secrets should be pulled from Infisical and how they should be exposed to your environment.

```yaml
config:
  infisical_project_id: "..."
  infisical_env: "dev"
  infisical_path: "/project/secrets"
  infisical_domain: "https://eu.infisical.com" # Required for non-US regions

secrets:
  - name: SSH_PRIVATE_KEY
    file: id_rsa           # Saved to /home/tazpod/secrets/id_rsa
    env: SSH_KEY_PATH      # export SSH_KEY_PATH=/home/tazpod/secrets/id_rsa
```

---

## 3. The Smart `pull` Workflow

The `tazpod pull` command is the "brain" of the sync process:

1.  **Enclave Check**: If the vault is locked, it prompts for the master passphrase and unlocks it first.
2.  **Session Check**: It attempts a lightweight sync. If Infisical reports "No valid session", TazPod automatically triggers the `login` flow.
3.  **Sync**:
    *   Generates a `.env-infisical` file in the Enclave containing all project variables.
    *   Downloads specific files defined in `secrets.yml`.
4.  **Auto-Save**: Immediately re-encrypts the vault to disk to ensure the latest sync state is persisted.

---

## 4. Security Defaults

*   **File Permissions**: All pulled secrets are automatically set to `0600` (Read/Write only by owner).
*   **No TTY Leak**: Environmental exports are handled via `__internal_env` and `eval` to prevent secret values from being printed to the terminal history.
*   **Region Support**: Full support for European (`eu.infisical.com`) and Self-Hosted instances via the `infisical_domain` config.

---
*Next: Explore the container images in [06-LAYERS-IMAGES.md](./06-LAYERS-IMAGES.md)*