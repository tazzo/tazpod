# TazPod - Project Context

## Project Overview
**TazPod** is a CLI tool written in Go that orchestrates secure, ephemeral, and containerized development environments. Its primary goal is to provide a "Zero Trust" workspace where sensitive secrets (managed via Infisical) are stored in an encrypted vault (`vault.tar.aes`) and accessed only within an isolated Linux Namespace ("Ghost Mode"), ensuring they never leak to the host or unauthorized processes.

## Technical Architecture
The system operates on three layers:
1.  **Orchestration (Host):** The `tazpod` CLI manages container lifecycles and project initialization.
2.  **Enclave (Kernel):** Uses tmpfs RAM Disk and AES-256-GCM encryption to create a secure memory space ("Ghost Mode").
3.  **Application (Container):** Modular Docker images providing toolstacks (Base, Infisical, K8s, Gemini).

## Key Components

### Directory Structure
*   `cmd/tazpod/`: Contains `main.go`, the entry point for the CLI.
*   `internal/`: Core logic packages.
    *   `engine/`: Logic for container orchestration.
    *   `vault/`: Handling of LUKS encryption and mounting.
    *   `utils/`: General utility functions.
*   `.tazpod/`: Default configuration and Docker build contexts.
    *   `config.yaml`: Default project configuration.
    *   `Dockerfile.*`: Definitions for the modular image layers.
*   `bin/`: Destination for compiled binaries.
*   `docs/`: Comprehensive documentation (Architecture, Installation, Usage).

### Configuration Files
*   `Taskfile.yml`: Defines build, test, and release tasks using the `task` tool.
*   `go.mod`: Go module definition (Go 1.23.2).
*   `secrets.yml`: Maps Infisical secrets to local files/env vars within the container.

## Development Workflow

### Prerequisites
*   Go 1.23+
*   Docker
*   Task (`go-task`)

### Build Commands
The project uses `Taskfile.yml` for automation:

*   **Build Binary:** `task build` (Compiles `bin/tazpod`)
*   **Build Docker Images:** `task docker:build` (Builds all layers: base, infisical, k8s, gemini)
*   **Release:** `task release` (Full cycle: build -> push -> tag -> release)
*   **Clean:** `task clean` (Removes artifacts)

### CLI Commands (`tazpod`)
*   `init`: Initialize a new TazPod project in the current directory.
*   `up`: Start the development container.
*   `ssh`: Enter the container shell.
*   `pull`: Unlock the vault and sync secrets from Infisical (enters "Ghost Mode").
*   `env`: Output environment variables for the current session.

## Conventions
*   **Language:** Go (Idiomatic, utilizing `internal` for private packages).
*   **Config:** YAML is used for all configuration (`config.yaml`, `secrets.yml`, `Taskfile.yml`).
*   **Security:** "Ghost Mode" logic is critical. Changes to namespace or encryption handling must be carefully reviewed to maintain zero-trust guarantees.
