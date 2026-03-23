# AGENTS.md — tazpod

Go CLI (v0.3.14+) that manages the developer enclave: LUKS-free AES-256-GCM vault,
Docker lifecycle, AWS SSO authentication, and S3-based Nomadic Identity.

## Build & Test

```bash
cd tazpod

# Build Linux/AMD64 binary → bin/tazpod
task build
# or manually:
GOOS=linux GOARCH=amd64 go build -ldflags "-X main.Version=$(cat VERSION)" -o bin/tazpod cmd/tazpod/main.go

# Run all tests
go test ./...
# Run specific package
go test ./internal/vault/...
# Run single test
go test -run TestName ./...

# Build all Docker layers (base → aws → k8s → ai)
task docker:build

# Release (prompts before tagging + pushing)
task release

# Clean
task clean
```

## Security Model

The vault stores secrets as `vault.tar.aes` (AES-256-GCM, PBKDF2 100k iterations).

On `unlock`:
- Contents extracted to a **tmpfs** RAM disk at `/home/tazpod/secrets`
- Auth paths (`~/.aws`, `~/.gemini`, `~/.claude`, `~/.pi`, `~/.omp`) bind-mounted from the RAM enclave
- Tools are stateless — no secrets persist on disk

On `lock`: unmounts and wipes RAM.

Environment management: `eval $(tazpod env)` / `tazpod lock`

## Docker Layer Hierarchy

```
base → aws → k8s → ai
```

Each layer is a separate image. `task docker:build` builds them in dependency order.

## Nomadic Identity (S3)

Identity sync targets S3:
- Bucket: `tazlab-storage`
- Region: `eu-central-1`
- Vault path: `tazpod/vault/vault.tar.aes`
- Identity path: `tazpod/identities/global.tar.gz`

## Code Layout

```
cmd/tazpod/main.go    entry point
internal/vault/       AES-256-GCM vault logic
internal/docker/      container lifecycle
internal/utils/       S3 and generic utils
VERSION               version string, injected via -ldflags
Taskfile.yml          task runner targets
```

## Conventions

- Version read from `VERSION` file, never hardcoded
- Table-driven tests, one `_test.go` per package
- GitHub push: extract token from `/home/tazpod/secrets/github-token`, use
  `git -c credential.helper="!f() { echo 'username=x-access-token'; echo \"password=${TOKEN}\"; }; f" push origin <branch>`
