package main

import (
	"fmt"
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
	vault.Save("")
	fmt.Println("💾 Vault content saved and encrypted to disk.")
}

func login() {
	fmt.Println("🔑 Authenticating with AWS SSO...")
	cmd := execCommand("aws", "sso", "login", "--profile", cfg.AwsSso.Profile)
	if err := cmd.Run(); err != nil {
		logger.Error("Login failed", "error", err)
	} else {
		fmt.Println("✅ Logged in successfully.")
	}
}

func loadConfigs() {
	data, err := os.ReadFile(ConfigPath)
	if os.IsNotExist(err) {
		return // no config yet, normal on first run
	}
	if err != nil {
		logger.Warn("Could not read config", "path", ConfigPath, "error", err)
		return
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		logger.Error("Invalid config", "path", ConfigPath, "error", err)
		os.Exit(1)
	}
}

// loadVaultAWSCredentials carica le credenziali AWS dal vault nell'env.
func loadVaultAWSCredentials() {
	if !utils.IsMounted(vault.MountPath) {
		return
	}
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join(vault.MountPath, name))
		if err != nil {
			return ""
		}
		v := strings.TrimSpace(string(data))
		for len(v) >= 2 && ((v[0] == '\'' && v[len(v)-1] == '\'') || (v[0] == '"' && v[len(v)-1] == '"')) {
			v = v[1 : len(v)-1]
		}
		return v
	}

	if key := read("aws_access_key_id"); key != "" {
		os.Setenv("AWS_ACCESS_KEY_ID", key)
	}
	if secret := read("aws_secret_access_key"); secret != "" {
		os.Setenv("AWS_SECRET_ACCESS_KEY", secret)
	}
}
