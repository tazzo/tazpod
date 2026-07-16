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

func up() {
	if cfg.Mode == "lxc" {
		fmt.Println("⚠️  'up' is not available in LXC mode — the container is always running.")
		fmt.Println("   SSH into it directly: ssh tazpod@<IP>")
		return
	}

	if cfg.Image == "" {
		logger.Error("No image defined in config. Run 'tazpod init' or check .tazpod/config.yaml")
		return
	}

	ensureContainerUp()
	fmt.Println("☁️  TazPod is up.")
}

func down() {
	if cfg.Mode == "lxc" {
		fmt.Println("⚠️  'down' is not available in LXC mode. Use terraform destroy to tear down the CT.")
		return
	}

	fmt.Printf("🛑 Stopping container %s...\n", cfg.ContainerName)
	exec.Command("docker", "stop", cfg.ContainerName).Run()
	exec.Command("docker", "rm", cfg.ContainerName).Run()
	fmt.Println("✅ TazPod stopped.")
}

func enter() {
	if cfg.Mode == "lxc" {
		fmt.Println("⚠️  'enter' is not available in LXC mode. Use: ssh tazpod@<IP>")
		return
	}
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
	cmd := exec.Command("bash", "-li")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Run()
}

func execInContainer(command string) bool {
	ensureContainerUp()
	cmd := exec.Command("docker", "exec", "-it", cfg.ContainerName, "bash", "-ic", command)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run() == nil
}

func smartEntry() {
	if cfg.Mode == "lxc" {
		smartEntryLxc()
		return
	}

	if _, err := os.Stat(".tazpod"); os.IsNotExist(err) {
		if !askYN("📂 No TazPod project found here. Initialize it now?") {
			return
		}
		initProject("docker")
		loadConfigs()
	}

	if cfg.ContainerName == "" {
		logger.Error("No container_name defined in config. Run 'tazpod init' or check .tazpod/config.yaml")
		return
	}

	ensureContainerUp()
	enterShellInContainer()
}

func enterShellInContainer() {
	cmd := exec.Command("docker", "exec", "-it", cfg.ContainerName, "bash", "-li")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Run()
}

func ensureContainerUp() {
	if cfg.Mode == "lxc" {
		return
	}

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
	fmt.Printf("🛠️  Creating container %s (might pull image first)...\n", cfg.ContainerName)
	args := []string{"run", "-d", "--name", cfg.ContainerName,
		"--network", "host",
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
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.Error("Failed to create container", "error", err)
		os.Exit(1)
	}
}

func smartEntryLxc() {
	if _, err := os.Stat(".tazpod"); os.IsNotExist(err) {
		if !askYN("📂 No TazPod project found here. Initialize it now?") {
			return
		}
		initProject("lxc")
		loadConfigs()
	}

	enterShell()
}

func execLocal(command string) bool {
	cmd := exec.Command("bash", "-c", command)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run() == nil
}

func updateImage() {
	if cfg.Mode == "lxc" {
		fmt.Println("⚠️  'pull image' is not available in LXC mode — binaries are deployed via Ansible, not Docker images.")
		return
	}
	if cfg.Image == "" {
		logger.Error("No image configured", "config", ConfigPath)
		return
	}

	fmt.Printf("🐳 Pulling image %s...\n", cfg.Image)
	cmd := execCommand("docker", "pull", cfg.Image)
	if err := cmd.Run(); err != nil {
		logger.Error("Image pull failed", "image", cfg.Image, "error", err)
		return
	}
	fmt.Println("✅ Image updated.")
}

func lock() {
	if cfg.Mode == "lxc" || utils.CheckInside() {
		fmt.Println("🔒 Locking GPG Agent (revoking cached passphrases)...")
		exec.Command("gpgconf", "--kill", "gpg-agent").Run()
		fmt.Println("✅ Cache wiped.")
		return
	}

	// Docker host mode -> forward to container
	if cfg.ContainerName == "" {
		logger.Error("No container_name defined in config.")
		return
	}

	// Check if container is running
	running, _ := exec.Command("docker", "ps", "-q", "--filter", "name="+cfg.ContainerName).Output()
	if len(strings.TrimSpace(string(running))) > 0 {
		fmt.Println("🔒 Revoking cached GPG keys inside container...")
		exec.Command("docker", "exec", cfg.ContainerName, "gpgconf", "--kill", "gpg-agent").Run()
		fmt.Println("✅ Cache wiped.")
	} else {
		fmt.Println("⚠️  Container is not running — cache is already empty.")
	}
}
