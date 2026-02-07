# Secure Memory Isolation: The RAM Enclave ☁️

In version 0.2.0, TazPod has transitioned from kernel-level "Ghost Mode" (Namespaces) to an application-level **RAM Enclave**. This provides similar security benefits with significantly better performance and cross-platform reliability.

## 1. The RAM Boundary (tmpfs)

TazPod leverages **tmpfs**, a Linux temporary filesystem that resides entirely in volatile memory.

*   **Zero Persistence**: Data in tmpfs is never committed to the physical drive. If the power is lost or the container stops, the secrets are gone.
*   **Encrypted Sync**: The only way secrets survive between container restarts is by being explicitly "Saved" (re-encrypted) into the `vault.tar.aes` file.

---

## 2. Bridging Auth (The Bind Strategy) 🔗

A development environment is useless if your tools (Infisical, Gemini, Git) can't see the secrets. TazPod uses **Bind Mounting** to bridge the RAM Enclave into your home directory.

| Real Location (RAM) | Target Path (Home) | Tool |
| :--- | :--- | :--- |
| `/home/tazpod/secrets/.infisical` | `~/.infisical` | Infisical CLI |
| `/home/tazpod/secrets/infisical-keyring` | `~/infisical-keyring` | Infisical Auth |
| `/workspace/.tazpod/.gemini` | `~/.gemini` | Gemini AI (Persistent) |

### The "Clean Table" Policy
Before هر mount, TazPod executes a `rm -rf` on the target path. This ensures that old symlinks or plaintext files are purged before the secure RAM enclave is mapped over them.

---

## 3. Environment Variable Cleanup 🧹

Variables like `GITHUB_TOKEN` or `KUBECONFIG` often point to files within the RAM Enclave. Leaving these set after the enclave is destroyed creates "Ghost Variables" that point to non-existent paths.

TazPod solves this via its **Smart Env Function**:

1.  **Unlock**: CLI outputs `export VAR="/home/tazpod/secrets/..."`.
2.  **Lock**: CLI outputs `unset VAR`.
3.  **Bash Integration**: The `.bashrc` automatically `eval`s these outputs, ensuring your shell environment is always in sync with the vault state.

---

## 4. Portability: Host vs Container

Because TazPod v0.2.0 uses standard Docker volume mounts and tmpfs, it works seamlessly across:
*   **Native Linux** (Ubuntu, Debian, Arch).
*   **WSL2** (Windows Subsystem for Linux).
*   **macOS** (via Docker Desktop / OrbStack).

The security model remains consistent: Secrets are encrypted at rest on the host disk and only decrypted into the container's volatile memory.

---
*Next: Learn how we manage secrets in [05-SECRETS-INFISICAL.md](./05-SECRETS-INFISICAL.md)*