package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"tazpod/internal/utils"
)

func up() {
	if cfg.Image == "" {
		fmt.Println("❌ No image defined in config. Run 'tazpod init' or check .tazpod/config.yaml")
		return
	}

	ensureContainerUp()

	// Spawn sync daemon
	exec.Command("tazpod", "__internal_sync_daemon").Start()
	fmt.Println("☁️  TazPod is up and syncing.")
}

func down() {
	fmt.Printf("🛑 Stopping container %s...\n", cfg.ContainerName)
	exec.Command("docker", "stop", cfg.ContainerName).Run()
	exec.Command("docker", "rm", cfg.ContainerName).Run()
	fmt.Println("✅ TazPod stopped.")
}

func enter() {
	smartEntry()
}

func smartEntry() {
	ensureContainerUp()
	fmt.Printf("🚀 Entering %s...\n", cfg.ContainerName)
	cmd := exec.Command("docker", "exec", "-it", "-w", "/workspace", cfg.ContainerName, "/bin/bash")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ Session ended with error: %v\n", err)
	} else {
		fmt.Println("\n🔒 Session ended. Securing identity...")
	}
	lock()
}

func ensureContainerUp() {
	// Check if container exists
	out, _ := exec.Command("docker", "ps", "-a", "--filter", "name="+cfg.ContainerName, "--format", "{{.Names}}").Output()
	if strings.TrimSpace(string(out)) == cfg.ContainerName {
		// Check if running
		running, _ := exec.Command("docker", "ps", "--filter", "name="+cfg.ContainerName, "--format", "{{.Names}}").Output()
		if strings.TrimSpace(string(running)) != cfg.ContainerName {
			exec.Command("docker", "start", cfg.ContainerName).Run()
		}
		return
	}

	cwd, _ := os.Getwd()
	fmt.Printf("🛠️  Creating container %s...\n", cfg.ContainerName)
	args := []string{"run", "-d", "--name", cfg.ContainerName,
		"-v", cwd + ":/workspace",
		"-v", filepath.Join(os.Getenv("HOME"), ".ssh") + ":/home/tazpod/.ssh:ro",
		"-e", "HOST_CWD=" + cwd,
		cfg.Image, "sleep", "infinity"}

	cmd := exec.Command("docker", args...)
	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ Failed to create container: %v\n", err)
		os.Exit(1)
	}
}

func setupStorage() {
	s3, err := utils.NewS3Client("", cfg.AwsSso.Profile)
	if err != nil {
		fmt.Printf("❌ S3 Client error: %v\n", err)
		return
	}

	fmt.Println("☁️  Setting up S3 path tazpod/vault/...")
	if err := s3.CreateBucket(); err != nil {
		fmt.Printf("❌ Setup failed: %v\n", err)
		return
	}
	fmt.Println("✅ S3 storage ready.")
}
