# Secure Memory Isolation: The RAM Enclave ☁️

In version 0.3.x, TazPod uses an application-level **RAM Enclave**. This provides security benefits with significantly better performance and cross-platform reliability.

## 1. The RAM Boundary (tmpfs)

TazPod leverages **tmpfs**, a Linux temporary filesystem that resides entirely in volatile memory.

*   **Zero Persistence**: Data in tmpfs is never committed to the physical drive. If the power is lost or the container stops, the secrets are gone.
*   **Encrypted Sync**: The only way secrets survive between container restarts is by being explicitly "Saved" (re-encrypted) into the `vault.tar.aes` file.

---

## 2. Bridging Auth (The Bind Strategy) 🔗

A development environment is useless if your tools (AWS CLI, Gemini, Git) can't see the secrets. TazPod uses **Bind Mounting** to bridge the RAM Enclave into your home directory.

| Real Location (RAM) | Target Path (Home) | Tool |
| :--- | :--- | :--- |
| `/home/tazpod/secrets/.aws` | `~/.aws` | AWS CLI / SSO |
| `/home/tazpod/secrets/.gemini` | `~/.gemini` | Gemini AI (Persistent Config) |
| `/home/tazpod/secrets/.claude` | `~/.claude` | Claude AI |
| `/home/tazpod/secrets/.pi` | `~/.pi` | Pi Coding Agent |
| `/home/tazpod/secrets/.omp` | `~/.omp` | Oh My Posh |

### The "Clean Table" Policy
Before any mount, TazPod executes a `rm -rf` on the target path. This ensures that old symlinks or plaintext files are purged before the secure RAM enclave is mapped over them.

---

## 3. Environment Variable Cleanup 🧹

Variables like `GITHUB_TOKEN` or `KUBECONFIG` often point to files within the RAM Enclave. Leaving these set after the enclave is destroyed creates "Ghost Variables" that point to nowhere. TazPod automatically cleans up the environment when locking the vault.

---
*Next: Manage secrets with AWS SSO in [05-SECRETS-AWS.md](./05-SECRETS-AWS.md)*
