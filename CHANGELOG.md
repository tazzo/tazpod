# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.30] - 2026-05-20

### Added
- Antigravity CLI native installation in the AI Docker layer (`Dockerfile.ai`).
- Persisted `.antigravity` and `.antigravitycli` settings and trust registry directories across container restarts in `.bashrc`.

## [0.3.16] - 2026-03-27

### Added
- Structured logging using `log/slog` across all CLI commands.
- Global `--debug` flag to enable debug level logs on stderr.
- Sync daemon now logs to `/tmp/tazpod-sync.log` with improved observability and SIGTERM handling.
- Docker layer pinning in CI and Taskfile using `ARG BASE_VERSION`.
- `CHANGELOG.md` to track project evolution.

### Changed
- Refactored `cmd/tazpod/main.go` into domain-specific files (`help.go`, `lifecycle.go`, `sync.go`, etc.).
- `tazpod init` now avoids creating empty provider blocks in `config.yaml`.
- Error messages moved from stdout to stderr via `logger.Error`.

### Removed
- Dead code related to "Nomadic Identity" (push/pull identity, PackageIdentity).
- Unused `identity` subcommands from `push` and `pull`.

## [0.3.15] - 2026-03-24

### Added
- `pi-mcp-adapter` to the AI Docker layer.
- `tailscale` tools to the Base Docker layer.

## [0.3.14] - 2026-03-22

### Added
- Full AWS SSO (IAM Identity Center) bootstrap flow.
- S3-based Vault synchronization.
- RAM-backed volatile enclave for secrets (`~/secrets`).

### Fixed
- Push/pull vault operations now use CWD-relative paths.

## [0.3.12] - 2026-03-20

### Fixed
- `smartEntry` TTY handling and host/container environment separation.
