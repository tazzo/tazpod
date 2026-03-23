# TazPod in Kubernetes: Remote Enclave ☸️

TazPod v0.3.x is designed to be highly portable. While it primarily runs locally, the architecture is ready for **Remote Development** inside a Kubernetes cluster.

## 1. Remote Architecture

In a remote scenario, the TazPod Pod acts as your primary compute engine.

*   **Provider Logic**: The CLI is being extended to support `--remote`.
*   **Compute**: The container runs as a Pod in your cluster.
*   **Storage**: A **Persistent Volume Claim (PVC)** is used to store your `vault.tar.aes` file.
*   **Decryption**: Cryptographic operations happen inside the Pod's RAM, keeping secrets isolated from the node's filesystem.

---

## 2. Remote Workflow (Roadmap)

1.  **Deploy**: `tazpod up --remote` applies a manifest to your cluster.
2.  **Access**: `tazpod enter` uses `kubectl exec` or a secure Tailscale tunnel (planned) to provide a TTY.
3.  **Sync**: Files are synced between your local IDE and the Remote Pod via `rsync` over SSH.

---

## 3. Remote Security Requirements

To maintain the same security level as the local environment, the cluster Pod requires specific capabilities:

```yaml
securityContext:
  privileged: true # Required for tmpfs and bind mounting
  capabilities:
    add: ["SYS_ADMIN"]
```

> **Security Note**: We recommend deploying Remote TazPods on dedicated, tainted nodes to prevent co-location with production workloads.

---

## 4. Current Limitations

*   **Privileged Pods**: Many enterprise clusters restrict privileged containers. Future versions will explore non-privileged RAM isolation.
*   **Latency**: TTY over `kubectl exec` can be slow. Real SSH over a VPN (Tailscale) is the recommended path for remote coding.

---
*Back to the main overview: [01-OVERVIEW.md](./01-OVERVIEW.md)*
