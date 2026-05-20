package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

var (
	ConfigPath = filepath.Join(".tazpod", "config.yaml")
	Version    = "dev" // Overridden at build time via ldflags
)

type VaultConfig struct {
	Retention int `yaml:"retention"`
}

type Config struct {
	Image         string                    `yaml:"image"`
	ContainerName string                    `yaml:"container_name"`
	User          string                    `yaml:"user"`
	GhostMode     bool                      `yaml:"ghost_mode"`
	Features      Features                  `yaml:"features"`
	AwsSso        AwsSsoConfig              `yaml:"aws_sso"`
	Vault         VaultConfig               `yaml:"vault"`
	Providers     map[string]ProviderConfig `yaml:"providers"`
}

type Features struct {
	Debug bool `yaml:"debug"`
}

type AwsSsoConfig struct {
	Profile string `yaml:"profile"`
	Bucket  string `yaml:"bucket"`
	Region  string `yaml:"region"`
}

type ProviderConfig struct {
	DBHost string `yaml:"db_host"`
}

var cfg Config

// execCommand is a helper to run a command with stdio attached
func execCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd
}
