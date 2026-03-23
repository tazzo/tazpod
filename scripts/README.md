# TazPod Scripts

This directory contains utility scripts for installing, building, and releasing the TazPod CLI and its associated Docker images.

## Scripts Overview

### 1. `install.sh`
**Purpose:** Universal installer for the TazPod CLI.  
**Usage:** `curl -sL <url> | bash` (or run locally).  
**What it does:**
- Detects the user's OS and Architecture.
- Downloads the latest `tazpod` binary from GitHub Releases.
- Installs it to `$HOME/.local/bin`.
- Verifies if the install location is in the system `$PATH`.

### 2. `publish-base.sh`
**Purpose:** Automated builder for the multi-layer Docker image stack.  
**Usage:** `./scripts/publish-base.sh [version]`  
**Example:** `./scripts/publish-base.sh v1.0.0`
**Prerequisites:** Docker logged in (`docker login`).  
**What it does:**
- Builds the images in dependency order:
  1. `tazpod-base` (OS, basic tools)
  2. `tazpod-aws` (AWS CLI + SSO tools)
  3. `tazpod-k8s` (Kubernetes tools)
  4. `tazpod-ai` (AI/Agentic layer)
- If a version argument is provided (e.g., `v1.0.0`), it tags the images with that version (e.g., `tazpod-base-v1.0.0`).
- Pushes all tagged images (latest and versioned) to the Docker registry.

> **Note**: `publish-base.sh` and `release.sh` are not present on disk. The equivalent operations are managed via `Taskfile.yml` (`task docker:build`, `task release`).

### 3. `release.sh`
**Purpose:** Automates the software release lifecycle.  
**Usage:** `./scripts/release.sh`  
**Prerequisites:** 
- `gh` (GitHub CLI) installed and authenticated.
- Go environment set up.
**What it does:**
- Prompts for a new version tag (e.g., `v1.0.1`).
- Compiles the Go binary for Linux/AMD64.
- Commits changes and pushes the new Git tag.
- Creates a GitHub Release and uploads the binary asset.

## Common Workflows

**To update the Docker images:**
```bash
./scripts/publish-base.sh
```

**To release a new CLI version:**
```bash
./scripts/release.sh
```
