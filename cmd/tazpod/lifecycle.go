package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"tazpod/internal/utils"
	"tazpod/internal/vault"
)

func up() {
	if cfg.Image == "" {
		logger.Error("No image defined in config. Run 'tazpod init' or check .tazpod/config.yaml")
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

func askYN(question string) bool {
	fmt.Printf("%s [y/N]: ", question)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

func enterShell() {
	fmt.Printf("🚀 Entering %s...\n", cfg.ContainerName)
	cmd := exec.Command("docker", "exec", "-it", "-w", "/workspace", cfg.ContainerName, "/bin/bash")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	if err := cmd.Run(); err != nil {
		logger.Error("Session ended with error", "error", err)
	} else {
		fmt.Println("\n🔒 Session ended. Securing identity...")
	}
	execInContainer("tazpod lock")
}

func execInContainer(command string) bool {
	cmd := exec.Command("docker", "exec", "-it",
		"-e", "AWS_CONFIG_FILE=/workspace/.tazpod/.aws/config",
		"-w", "/workspace",
		cfg.ContainerName, "bash", "-lc", command)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run() == nil
}

func smartEntry() {
	if _, err := os.Stat(".tazpod"); os.IsNotExist(err) {
		if !askYN("📂 No TazPod project found here. Initialize it now?") {
			return
		}
		initProject()
		loadConfigs()
	}

	if cfg.ContainerName == "" {
		logger.Error("No container_name defined in config. Run 'tazpod init' or check .tazpod/config.yaml")
		return
	}

	ensureContainerUp()

	cwd, _ := os.Getwd()
	localVault := filepath.Join(cwd, ".tazpod", "vault", "vault.tar.aes")
	containerUnlocked := exec.Command("docker", "exec", cfg.ContainerName, "mountpoint", "-q", vault.MountPath).Run() == nil

	if containerUnlocked {
		enterShell()
		return
	}

	if utils.FileExist(localVault) {
		if askYN("🔐 Local vault found. Unlock now?") {
			execInContainer("tazpod unlock")
		}
		enterShell()
		return
	}

	if askYN("🔑 No local vault found. Bootstrap now? (login + pull + unlock)") {
		if !execInContainer("tazpod login") {
			enterShell()
			return
		}
		if !execInContainer("tazpod pull vault") {
			enterShell()
			return
		}
		execInContainer("tazpod unlock")
	}

	enterShell()
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
		"--network", "host",
		"--cap-add", "SYS_ADMIN",
		"--cap-add", "NET_ADMIN",
		"--device", "/dev/net/tun",
		"--security-opt", "apparmor=unconfined",
		"--dns", "1.1.1.1",
		"--dns", "1.0.0.1",
		"-v", cwd + ":/workspace",
		"-v", filepath.Join(os.Getenv("HOME"), ".ssh") + ":/home/tazpod/.ssh:ro",
		"-e", "HOST_CWD=" + cwd,
		cfg.Image, "sleep", "infinity"}

	cmd := exec.Command("docker", args...)
	if err := cmd.Run(); err != nil {
		logger.Error("Failed to create container", "error", err)
		os.Exit(1)
	}
}

func setupStorage() {
	s3, err := utils.NewS3Client("", cfg.AwsSso.Profile)
	if err != nil {
		logger.Error("S3 Client error", "error", err)
		return
	}

	fmt.Println("☁️  Setting up S3 path tazpod/vault/...")
	if err := s3.CreateBucket(); err != nil {
		logger.Error("Setup failed", "error", err)
		return
	}
	fmt.Println("✅ S3 storage ready.")
}
