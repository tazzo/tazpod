# TazPod in Kubernetes: Remote Enclave Roadmap ☸️🚀

This document outlines the roadmap for deploying TazPod as a development pod directly inside a Kubernetes cluster. This evolution shifts TazPod from a local Docker wrapper to a **Remote Development Environment** that lives where your apps run.

---

## 1. The Vision

The goal is to extend the TazPod CLI to support a `provider` logic:
*   **Local Provider (Default)**: Uses local Docker Engine.
*   **K8s Provider**: Uses a Kubernetes cluster as the compute engine.

### User Workflow:
1.  **`tazpod up --remote`**: Generates a Pod manifest, applies it to the cluster, and waits for readiness.
2.  **`tazpod enter`**: Establishes a secure TTY session (via `kubectl exec` or SSH).
3.  **`tazpod unlock`**: Performs the same LUKS decryption logic, but inside the cluster pod.

---

## 2. Deployment Strategies

### Strategy A: The "Native Pod" (via Kubectl Exec)
This is the simplest way to get started. TazPod runs as a standard Pod, and the CLI uses the Kubernetes API to pipe stdin/stdout.

*   **Pros**:
    *   No extra network configuration (works as long as `kubectl` works).
    *   Zero cost (no LoadBalancers or NodePorts).
    *   Seamless integration with existing RBAC.
*   **Cons**:
    *   `kubectl exec` is not a real SSH session (can have TTY/color artifacts).
    *   Connections can be unstable for long-running Tmux sessions.
    *   Port forwarding required for every single service.

### Strategy B: The "SSH Enclave" (via VPN + Private IP)
The Pod runs an SSH daemon. The developer connects to the cluster's private network (Wireguard/Tailscale) and accesses the Pod via its ClusterIP or a dedicated Service.

*   **Pros**:
    *   **Real SSH**: Perfect support for Neovim, Tmux, and VS Code Remote SSH.
    *   **Performant**: Lower latency than API-wrapped streams.
    *   **Native Networking**: Access cluster services (DBs, APIs) via their internal DNS (`my-service.namespace.svc`) directly from the dev shell.
*   **Cons**:
    *   Requires a VPN/SDN (Tailscale/Wireguard) already configured.
    *   Requires managing SSH keys inside the Pod.

---

## 3. Technical Requirements & Roadmap

### Phase 1: The Manifest Template
TazPod needs to generate a specialized manifest. Key requirements:
*   **Privileged Mode**: Required for `losetup` and `cryptsetup` (LUKS) to work inside the container.
*   **Security Context**:
    ```yaml
    securityContext:
      privileged: true
      capabilities:
        add: ["SYS_ADMIN", "IPC_LOCK"]
    ```
*   **Persistence**: A `PersistentVolumeClaim` (PVC) must be mounted at `/workspace` to store the `.tazpod/vault/vault.img`.

### Phase 2: CLI Provider Logic
Update `main.go` to handle the remote lifecycle:
1.  **Context Detection**: Read `KUBECONFIG` to identify the target cluster.
2.  **Manifest Injection**: Use a Go template to create the Pod with the correct image (`tazpod-gemini`) and project tags.
3.  **Sync Logic**: Implement a "Pre-flight Sync" using `rsync` or `tar` over `kubectl exec` to ensure local code matches the Pod's `/workspace`.

### Phase 3: The "Ghost" Bridge in K8s
The `internal-ghost` logic remains identical, but we must ensure the K8s node has the necessary kernel modules loaded:
*   `dm_crypt`
*   `loop`
*   `tun` (if using internal VPN)

---

## 4. Security Considerations 🛡️

Running a **privileged pod** is a security trade-off. 
*   **Mitigation 1**: Use **Node Selectors** or **Taints** to ensure TazPod runs on dedicated "Dev Nodes" isolated from production workloads.
*   **Mitigation 2**: NetworkPolicies to restrict the Pod's ability to scan the entire cluster network by default.
*   **Mitigation 3**: The Vault remains the ultimate line of defense. Even if the Pod is compromised, the secrets are encrypted in the LUKS loop file.

---

## 5. Proposed CLI Commands

| Command | Action |
| :--- | :--- |
| `tazpod up --remote` | Deploy the Pod and PVC to the current K8s context. |
| `tazpod down --remote` | Delete the Pod (keeps the PVC). |
| `tazpod enter` | Automatically detects if a remote pod exists and uses `kubectl exec`. |
| `tazpod sync` | Bi-directional sync between local folder and remote Pod. |

---
