# Secrets Management with AWS SSO

TazPod's secrets system is built around a single encrypted artifact — `vault.tar.aes` — gated by AWS SSO. No long-term keys are stored anywhere on disk.

---

## 1. The TazPod Vault

`vault.tar.aes` is a tar archive encrypted with AES-256-GCM. The encryption key is derived from a master passphrase via PBKDF2 (SHA-256, 100,000 iterations).

The vault is the **bootstrap anchor**: it holds AWS credentials, SSH keys, dotfile configs, and any other secrets that must be available inside the container. Nothing in the system works without the vault being unlocked first.

At rest, the vault lives on S3 at:

```
s3://tazlab-storage/tazpod/vault/vault.tar.aes
```

---

## 2. AWS SSO as the Entry Point

`tazpod login` runs:

```
aws sso login --profile tazlab-bootstrap
```

This exchanges an SSO session for **temporary credentials valid for 8 hours**. No static access keys, no IAM user keys, no long-lived tokens stored on disk.

Once the SSO session is active, all S3 operations (`pull`, `push`) use those temporary credentials.

---

## 3. Bootstrap from Scratch (no local vault)

When there is no `vault.tar.aes` on the machine:

1. `tazpod login` — authenticate via AWS SSO
2. `tazpod pull vault` — download `vault.tar.aes` from S3 to `.tazpod/vault/`
3. `tazpod unlock` — decrypt vault into a 64 MB tmpfs, bind-mount `.aws`, run `SetupIdentity`

After step 3, all secrets are available in RAM and the container environment is ready.

`tazpod smartEntry` (invoked with no arguments) automates this entire sequence: it detects the absence of a local vault and runs login → pull vault → unlock before entering the container.

---

## 4. Normal Session Lifecycle

```
tazpod unlock        # decrypt vault → tmpfs, bind-mount .aws
# ... work inside container ...
tazpod save          # tar + gzip + AES-256-GCM → vault.tar.aes
tazpod push vault    # upload vault.tar.aes to S3
tazpod lock          # unmount .aws bind, unmount tmpfs (RAM zeroed)
```

A background sync daemon also runs `save` + `push vault` every 5 minutes while the vault is unlocked, ensuring work is not lost if the session ends unexpectedly.

---

## 5. AWS Enclave Bridge

The vault decrypts to a 64 MB tmpfs mount. The only bind mount performed is:

```
/home/tazpod/secrets/.aws  ↔  /home/tazpod/.aws
```

AWS credentials live exclusively in RAM for the duration of the unlocked session. When `tazpod lock` runs, the tmpfs is unmounted and the memory is zeroed. Nothing persists to the host filesystem in plaintext.

---

## 6. Rotation and Security Properties

| Property | Detail |
|---|---|
| SSO token lifetime | 8 hours, non-renewable without re-login |
| Long-term keys on disk | None |
| Vault encryption | AES-256-GCM + PBKDF2 (100k iterations) |
| Vault at rest | S3 object, protected by passphrase |
| Credentials in RAM | Only while vault is unlocked |

To rotate the vault passphrase: `unlock` with the current passphrase, `save` (which re-encrypts), update the passphrase, `save` again, `push vault`.

---

*Next: Explore the container images in [06-LAYERS-IMAGES.md](./06-LAYERS-IMAGES.md)*
