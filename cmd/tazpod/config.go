package main

import (
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var (
	ConfigPath = filepath.Join(".tazpod", "config.yaml")
	Version    = "dev" // Overridden at build time via ldflags
)

type GopassConfig struct {
	Store string `yaml:"store"`
}

type Config struct {
	Mode          string                    `yaml:"mode"`          // "docker" (default) o "lxc"
	Image         string                    `yaml:"image"`
	ContainerName string                    `yaml:"container_name"`
	User          string                    `yaml:"user"`
	GhostMode     bool                      `yaml:"ghost_mode"`
	Features      Features                  `yaml:"features"`
	Gopass        GopassConfig              `yaml:"gopass"`
	Providers     map[string]ProviderConfig `yaml:"providers"`
}

type Features struct {
	Debug bool `yaml:"debug"`
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

func loadConfigs() {
	data, err := os.ReadFile(ConfigPath)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		// Se il logger non è inizializzato, usiamo log di Go standard o fprint
		os.Stderr.WriteString("⚠️  Could not read config: " + err.Error() + "\n")
		return
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		os.Stderr.WriteString("❌ Invalid config: " + err.Error() + "\n")
		os.Exit(1)
	}
	if cfg.Mode == "" {
		cfg.Mode = "docker"
	}
}
