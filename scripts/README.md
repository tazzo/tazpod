# TazPod Scripts

## `install.sh`

Universal installer for the TazPod CLI.

```bash
curl -sSL https://raw.githubusercontent.com/tazzo/tazpod/master/scripts/install.sh | bash
```

Downloads the latest binary from GitHub Releases and installs it to `$HOME/.local/bin`.

## Build & Release

Build and release operations are managed via `Taskfile.yml`:

```bash
task build          # Compile Go binary for Linux/AMD64
task docker:build   # Build full Docker layer hierarchy (base → aws → k8s → ai)
task release        # Tag, commit, push and trigger GitHub release
task clean          # Remove build artifacts
```
