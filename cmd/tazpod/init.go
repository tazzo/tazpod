package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func initProject() {
	cwd, _ := os.Getwd()
	configDir := filepath.Join(cwd, ".tazpod")
	if _, err := os.Stat(configDir); err == nil {
		fmt.Printf("⚠️  TazPod is already initialized in %s\n", cwd)
		return
	}

	fmt.Printf("Initializing TazPod in %s...\n", cwd)
	os.MkdirAll(configDir, 0755)
	os.MkdirAll(filepath.Join(configDir, "vault"), 0755)

	// Create default config
	newCfg := Config{
		Image:         "tazzo/tazpod-ai:latest",
		ContainerName: filepath.Base(cwd) + "-lab",
		User:          "tazpod",
		GhostMode:     true,
		Features: Features{
			Debug: false,
		},
		Providers: make(map[string]ProviderConfig),
	}

	promptInitConfig(&newCfg)

	data, _ := yaml.Marshal(&newCfg)
	os.WriteFile(ConfigPath, data, 0644)

	fmt.Println("✅ project initialized. Check .tazpod/config.yaml")
}

func promptInitConfig(c *Config) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("🏠 Home DB Host (IP): ")
	dbHost, _ := reader.ReadString('\n')
	dbHost = strings.TrimSpace(dbHost)
	c.Providers["home"] = ProviderConfig{DBHost: dbHost}

	fmt.Print("☁️  AWS DB Host (IP): ")
	dbHost, _ = reader.ReadString('\n')
	dbHost = strings.TrimSpace(dbHost)
	c.Providers["aws"] = ProviderConfig{DBHost: dbHost}
}
