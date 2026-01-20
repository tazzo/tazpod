# TazPod Development Guidelines

This document contains guidelines for agentic coding agents working on the TazPod codebase.

## Project Overview

TazPod is a Zero-Trust Containerized Developer Environment built with Go 1.23.2 and Docker. The project provides secure, isolated development environments with encrypted vaults and comprehensive DevOps tooling.

## Build & Development Commands

### Docker Build Commands
```bash
# Full multi-layer build
./publish-base.sh

# Individual layer builds
docker build -t tazzo/tazlab.net:tazpod-base -f .tazpod/Dockerfile.base .
docker build -t tazzo/tazlab.net:tazpod-infisical -f .tazpod/Dockerfile.infisical .
docker build -t tazzo/tazlab.net:tazpod-k8s -f .tazpod/Dockerfile.k8s .
```

### Development Workflow
```bash
# Start development environment
./tazpod up

# Enter container
./tazpod enter
./tazpod ssh

# Inside container - unlock vault
tazpod unlock

# Stop environment
./tazpod down
```

### Testing
- No automated test suite - manual testing through CLI interface
- Test by running commands and verifying container behavior
- Security testing through vault encryption/decryption flows

## Code Style Guidelines

### Go Code Style
- Follow standard Go formatting (`go fmt`)
- Use standard Go error handling patterns
- Package organization: `internal/` for private packages
- Exported functions use CamelCase, internal functions use lowercase

### Import Organization
```go
import (
    // Standard library
    "fmt"
    "os"
    "os/exec"
    "syscall"
    
    // Local project imports
    "tazpod/internal/engine"
    "tazpod/internal/utils"
    "tazpod/internal/vault"
    
    // Third-party imports
    "golang.org/x/term"
    "gopkg.in/yaml.v3"
)
```

### Function Naming & Documentation
- Exported functions should have concise comments explaining purpose
- Use descriptive function names (e.g., `CheckInside`, `WaitForDevice`)
- Error messages should be user-friendly with emoji indicators
- Constants should be `UPPER_SNAKE_CASE`

### Error Handling
- Return errors as second return value
- Use `fmt.Printf` for user-facing error messages with emoji indicators
- System errors should include context about the failing command
- Use `os.Exit(1)` for fatal CLI errors

### CLI Interface Patterns
- Use switch statement for command routing in main()
- Group commands by context (Host vs Container commands)
- Provide help text with command descriptions
- Validate command context (e.g., `checkInside()` for container-only commands)

## Project Structure

```
tazpod/
├── cmd/tazpod/main.go        # CLI entry point
├── internal/
│   ├── engine/engine.go      # Docker container management
│   ├── utils/utils.go        # Common utilities
│   └── vault/vault.go        # Vault encryption/decryption
├── .tazpod/
│   ├── config.yaml           # Main configuration
│   ├── Dockerfile.base       # Base environment
│   ├── Dockerfile.infisical  # Secrets management
│   └── Dockerfile.k8s        # Kubernetes tools
├── dotfiles/                 # User environment config
└── docs/ARCHITECTURE.md      # Security documentation
```

## Configuration Management

### YAML Configuration
- Main config in `.tazpod/config.yaml`
- Use `gopkg.in/yaml.v3` for YAML parsing
- Environment-specific settings via Docker layers

### Secrets Management
- Use Infisical for secrets management
- Secrets mapped in `secrets.yml`
- Vault encryption/decryption in `internal/vault/`

## Security Considerations

- Always validate container context before executing commands
- Use privileged containers only when necessary
- Encrypt sensitive data in vaults
- Never log secrets or sensitive information
- Follow zero-trust principles throughout

## Development Tools Integration

### Shell Environment
- Modern aliases: `ls="eza --icons"`, `cat="bat"`, `find="fd"`
- Git aliases: `g="git"`, `lg="lazygit"`
- DevOps aliases: `k="kubectl"`, `ctx="kubectx"`

### Editor Configuration
- Neovim + LazyVim configuration in `dotfiles/.config/nvim/`
- Lua formatting with 2-space indentation
- 120-character line width limit

## Common Patterns

### Command Execution
```go
// For commands with output
utils.RunCmd("docker", "build", "-f", Dockerfile, "-t", ImageName, ".")

// For commands returning output
output := utils.RunOutput("docker", "ps")

// For commands with stdin input
result, err := utils.RunWithStdin(input, "command", "arg1", "arg2")
```

### File System Operations
```go
// Check file existence
if utils.FileExist(path) { ... }

// Check mount status
if utils.IsMounted(path) { ... }

// Wait for device
utils.WaitForDevice("/dev/mapper/vault")
```

### Container Context Validation
```go
func checkInside() {
    if !utils.CheckInside() {
        fmt.Println("❌ This command must be run INSIDE the TazPod container.")
        os.Exit(1)
    }
}
```

## Adding New Features

1. Determine if feature belongs in host or container context
2. Add command to appropriate switch case in `cmd/tazpod/main.go`
3. Implement logic in appropriate `internal/` package
4. Add context validation if container-only command
5. Update help text with new command description
6. Test manually through CLI interface

## Important Notes

- This is a production tool, not a typical application
- Security and reliability are paramount
- No automated tests - manual testing required
- Focus on developer experience and productivity
- Maintain backward compatibility for existing workflows