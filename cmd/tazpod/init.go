package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"tazpod/internal/utils"

	"gopkg.in/yaml.v3"
)

func initProject(mode string) {
	cwd, _ := os.Getwd()
	configDir := filepath.Join(cwd, ".tazpod")
	if _, err := os.Stat(configDir); err == nil {
		logger.Warn("TazPod is already initialized", "path", cwd)
		return
	}

	fmt.Printf("Initializing TazPod in %s...\n", cwd)
	os.MkdirAll(configDir, 0755)

	// Create agent folders inside .tazpod
	agentsDirs := []string{".pi", ".omp", ".gemini", ".claude", ".aws", ".opencode", ".herdr", "vault", "keyrings"}
	for _, d := range agentsDirs {
		os.MkdirAll(filepath.Join(configDir, d), 0755)
	}

	// Auto-detect o usa mode esplicita
	if mode == "" {
		if utils.CheckInside() {
			mode = "docker"
		} else {
			fmt.Print("Deployment mode [docker/lxc] (docker): ")
			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))
			if input == "lxc" {
				mode = "lxc"
			} else {
				mode = "docker"
			}
		}
	}

	// Create default config — Docker ha Image e ContainerName, LXC no
	newCfg := Config{
		Mode:      mode,
		User:      "tazpod",
		GhostMode: true,
		Features: Features{
			Debug: false,
		},
		Gopass: GopassConfig{
			Store: "/workspace/tazlab-secrets",
		},
		Providers: make(map[string]ProviderConfig),
	}
	if mode != "lxc" {
		newCfg.Image = "tazzo/tazpod-ai:latest"
		newCfg.ContainerName = filepath.Base(cwd) + "-lab"
	}

	promptInitConfig(&newCfg)

	data, _ := yaml.Marshal(&newCfg)
	os.WriteFile(ConfigPath, data, 0644)

	fmt.Printf("✅ project initialized (%s mode). Check .tazpod/config.yaml\n", mode)
}

func promptInitConfig(c *Config) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("🏠 Home DB Host (IP): ")
	dbHost, _ := reader.ReadString('\n')
	dbHost = strings.TrimSpace(dbHost)
	if dbHost != "" {
		c.Providers["home"] = ProviderConfig{DBHost: dbHost}
	}

	fmt.Print("☁️  AWS DB Host (IP): ")
	dbHost, _ = reader.ReadString('\n')
	dbHost = strings.TrimSpace(dbHost)
	if dbHost != "" {
		c.Providers["aws"] = ProviderConfig{DBHost: dbHost}
	}

	fmt.Printf("🔑 Gopass Secrets Store Path [%s]: ", c.Gopass.Store)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		c.Gopass.Store = input
	}
}
