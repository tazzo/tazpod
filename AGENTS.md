# AGENTS.md — tazpod

Go CLI (v0.2+) that manages the developer enclave: LUKS-free AES-256-GCM vault,
Docker lifecycle, Infisical secrets, and S3-based Nomadic Identity.

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

# Build all Docker layers (base → infisical → k8s → ai)
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
- Auth paths (`~/.infisical`, `~/.gemini`, `~/.claude`, `~/.pi`, `~/.omp`) bind-mounted from the RAM enclave
- Tools are stateless — no secrets persist on disk

On `lock`: unmounts and wipes RAM.

Environment management: `eval $(tazpod env)` / `tazpod lock`

## Docker Layer Hierarchy

```
base → infisical → k8s → ai → ... + Pi (pi), oh-my-pi (omp)
```

Each layer is a separate image. `task docker:build` builds them in dependency order.

## Nomadic Identity (S3)

Identity sync targets S3:
- PGO backup path: `/pgbackrest/repo1/`
- TazPod identity path: `/pgbackrest/tazpod/` (planned)

Bucket: `tazlab-longhorn`, region: `eu-central-1`.

## Code Layout

```
cmd/tazpod/main.go    entry point
internal/vault/       AES-256-GCM vault logic
internal/docker/      container lifecycle
VERSION               version string, injected via -ldflags
Taskfile.yml          task runner targets
```

## Conventions

- Version read from `VERSION` file, never hardcoded
- Table-driven tests, one `_test.go` per package
- GitHub push: extract token from `/home/tazpod/secrets/github-token`, use
  `git -c http.extraheader="Authorization: Basic $(echo -n x-access-token:${TOKEN} | base64)" push`
