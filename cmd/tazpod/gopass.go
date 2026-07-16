package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"tazpod/internal/utils"
)

func gopassCmd() {
	// 1. Host-to-Container Forwarding check
	if cfg.Mode != "lxc" && !utils.CheckInside() {
		if cfg.ContainerName == "" {
			logger.Error("No container_name defined in config.")
			return
		}
		ensureContainerUp()
		fmt.Println("➡️  Forwarding gopass initialization inside container...")
		execInContainer("tazpod gopass")
		return
	}

	// 2. Local execution (Inside container or LXC mode)
	reader := bufio.NewReader(os.Stdin)
	defaultStore := cfg.Gopass.Store
	if defaultStore == "" {
		defaultStore = "/workspace/tazlab-secrets"
	}

	fmt.Printf("📂 Enter path to your secrets directory [%s]: ", defaultStore)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		input = defaultStore
	}

	// Resolve absolute path
	absPath := input
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Clean(absPath)
	}

	// 3. Verify directory exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		fmt.Printf("❌ Secrets directory not found at: %s\n", absPath)
		fmt.Println("   Make sure it is cloned or mounted correctly in the workspace.")
		return
	}

	// 4. Import GPG private keys via shell (bash wildcard expansion)
	keysWildcard := filepath.Join(absPath, "gpg-keys", "*.asc")
	fmt.Printf("🔑 Importing GPG private keys from %s...\n", keysWildcard)
	
	// Eseguiamo tramite bash -c per espandere il wildcard
	cmd := exec.Command("bash", "-c", fmt.Sprintf("gpg --import %s", keysWildcard))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("⚠️  No GPG keys found or import completed with errors. Proceeding anyway...")
	} else {
		fmt.Println("✅ GPG keys imported successfully.")
	}

	// 5. Initialize gopass store via symlink
	home, _ := os.UserHomeDir()
	storeRoot := filepath.Join(home, ".local", "share", "gopass", "stores")
	os.MkdirAll(storeRoot, 0755)

	targetLink := filepath.Join(storeRoot, "root")
	fmt.Printf("🔗 Symlinking %s -> %s...\n", absPath, targetLink)
	
	// Rimuove link o directory pre-esistente per evitare ricorsione
	os.Remove(targetLink)
	if err := os.Symlink(absPath, targetLink); err != nil {
		logger.Error("Failed to create symlink", "error", err)
		return
	}

	fmt.Println("\n🎉 Gopass store configured successfully!")
	fmt.Println("   Test it by running: gopass list")
}
