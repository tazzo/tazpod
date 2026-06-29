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
	os.MkdirAll(filepath.Join(configDir, "vault"), 0755)

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
		AwsSso: AwsSsoConfig{
			Profile: "tazlab",
			Bucket:  utils.DefaultBucket,
			Region:  utils.DefaultRegion,
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

	fmt.Printf("☁️  S3 Bucket [%s]: ", c.AwsSso.Bucket)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		c.AwsSso.Bucket = input
	}

	fmt.Printf("🌍  S3 Region [%s]: ", c.AwsSso.Region)
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		c.AwsSso.Region = input
	}

	fmt.Printf("🔑 AWS SSO Profile [%s]: ", c.AwsSso.Profile)
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		c.AwsSso.Profile = input
	}
}
