package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"tazpod/internal/utils"
	"tazpod/internal/vault"
	"gopkg.in/yaml.v3"
)

func unlock() {
	vault.Unlock()
	fmt.Println("🔓 Vault unlocked to ~/secrets (RAM only).")

	// AWS SSO bridge: bind ~/secrets/.aws to ~/.aws
	home, _ := os.UserHomeDir()
	source := filepath.Join(vault.MountPath, ".aws")
	target := filepath.Join(home, ".aws")

	os.MkdirAll(source, 0700)
	execCommand("sudo", "mount", "--bind", source, target).Run()

	fmt.Println("☁️  AWS Enclave Bridge active.")
}

func lock() {
	// Unmount AWS bridge first
	home, _ := os.UserHomeDir()
	target := filepath.Join(home, ".aws")
	if utils.IsMounted(target) {
		execCommand("sudo", "umount", "-l", target).Run()
	}

	vault.Lock()
	fmt.Println("🔒 Vault locked and wiped.")
}

func save() {
	if _, err := vault.Save(""); err != nil {
		slog.Warn("Save completed with errors", "error", err)
	}
	fmt.Println("💾 Vault content saved and encrypted to disk.")
}

func login() {
	if !awsProfileExists(cfg.AwsSso.Profile) {
		fmt.Printf("🔧 AWS profile '%s' not found in ~/.aws/config.\n", cfg.AwsSso.Profile)
		if askYN("Run 'aws configure sso' to set it up now?") {
			cmd := execCommand("aws", "configure", "sso", "--profile", cfg.AwsSso.Profile)
			if err := cmd.Run(); err != nil {
				logger.Error("AWS SSO configuration failed", "error", err)
				return
			}
		} else {
			return
		}
	}

	fmt.Println("🔑 Authenticating with AWS SSO...")
	cmd := execCommand("aws", "sso", "login", "--profile", cfg.AwsSso.Profile)
	if err := cmd.Run(); err != nil {
		logger.Error("Login failed", "error", err)
	} else {
		fmt.Println("✅ Logged in successfully.")
	}
}

func awsProfileExists(profile string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(filepath.Join(home, ".aws", "config"))
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "["+"profile "+profile+"]")
}

func loadConfigs() {
	data, err := os.ReadFile(ConfigPath)
	if os.IsNotExist(err) {
		cfg.Vault.Retention = 50
		return
	}
	if err != nil {
		logger.Warn("Could not read config", "path", ConfigPath, "error", err)
		cfg.Vault.Retention = 50
		return
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		logger.Error("Invalid config", "path", ConfigPath, "error", err)
		os.Exit(1)
	}
	if cfg.Vault.Retention <= 0 {
		cfg.Vault.Retention = 50
	}
}

func list() {
	subarg := ""
	if len(os.Args) > 2 {
		subarg = os.Args[2]
	}
	switch subarg {
	case "vault-history":
		listVaultHistory()
	default:
		fmt.Println("Usage: tazpod list vault-history")
	}
}

// loadVaultAWSCredentials carica le credenziali AWS dal vault nell'env.
func loadVaultAWSCredentials() {
	if !utils.IsMounted(vault.MountPath) {
		return
	}
	read := func(names ...string) string {
		for _, name := range names {
			data, err := os.ReadFile(filepath.Join(vault.MountPath, name))
			if err != nil {
				continue
			}
			v := strings.TrimSpace(string(data))
			for len(v) >= 2 && ((v[0] == '\'' && v[len(v)-1] == '\'') || (v[0] == '"' && v[len(v)-1] == '"')) {
				v = v[1 : len(v)-1]
			}
			if v != "" {
				return v
			}
		}
		return ""
	}

	if key := read("aws_access_key_id", "aws-access-key-id"); key != "" {
		os.Setenv("AWS_ACCESS_KEY_ID", key)
	}
	if secret := read("aws_secret_access_key", "aws-secret-access-key"); secret != "" {
		os.Setenv("AWS_SECRET_ACCESS_KEY", secret)
	}
}
