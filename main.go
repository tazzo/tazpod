package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
)

// Configuration constants
const (
	ContainerName = "tazpod-lab"
	ImageName     = "tazpod-engine:local"
	VaultDir      = "/workspace/.tazpod-vault"
	VaultPath     = VaultDir + "/vault.img"
	MountPath     = "/home/tazpod/secrets"
	MapperName    = "tazpod_vault"
	VaultSizeMB   = "512"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: tazpod [up|down|enter|ssh|unlock|lock|reinit]")
		os.Exit(1)
	}

	cmd := os.Args[1]

	// Logic branching based on context
	switch cmd {
	case "up":
		up()
	case "down":
		down()
	case "enter", "ssh":
		enter()
	case "unlock":
		checkInside()
		unlock()
	case "lock":
		checkInside()
		lock()
	case "reinit":
		checkInside()
		reinit()
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		os.Exit(1)
	}
}

// --- HOST COMMANDS (Manage the container from outside) ---

func up() {
	fmt.Println("🏗️  Ensuring TazPod Engine Image...")
	// We assume Dockerfile.base is in the same dir as the binary during 'up'
	// or we can hardcode the absolute path if needed.
	runCmd("docker", "build", "-f", "Dockerfile.base", "-t", ImageName, ".")
	
	fmt.Println("🛑 Cleaning instances...")
	exec.Command("docker", "rm", "-f", ContainerName).Run()
	
cwd, _ := os.Getwd()
	fmt.Printf("🚀 Starting TazPod in %s...\n", cwd)
	
	runCmd("docker", "run", "-d", 
		"--name", ContainerName, 
		"--privileged", 
		"--network", "host", 
		"-v", cwd+":/workspace", 
		"-w", "/workspace", 
		ImageName, "sleep", "infinity")
	
	fmt.Println("✅ TazPod is alive.")
	fmt.Println("👉 Run 'tazpod enter' to get inside.")
}

func down() {
	fmt.Println("🧹 Shutting down TazPod...")
	runCmd("docker", "rm", "-f", ContainerName)
	fmt.Println("✅ Done.")
}

func enter() {
	fmt.Println("🚪 Entering TazPod...")
	// Use syscall.Exec to hand over control to the docker exec process
	binary, _ := exec.LookPath("docker")
	args := []string{"docker", "exec", "-it", ContainerName, "bash"}
	env := os.Environ()
	syscall.Exec(binary, args, env)
}

// --- CONTAINER COMMANDS (Manage security from inside) ---

func unlock() {
	if isMounted(MountPath) {
		fmt.Println("✅ Vault already active.")
		return
	}

	// Clean up any stale mapper from previous sessions
	exec.Command("sudo", "cryptsetup", "close", MapperName).Run()
	exec.Command("sudo", "dmsetup", "remove", "-f", MapperName).Run()

	var passphrase string
	if !fileExist(VaultPath) {
		fmt.Println("🆕 Vault not found. Creating NEW persistent vault...")
		os.MkdirAll(VaultDir, 0755)
		for {
			fmt.Print("📝 Define Master Passphrase: ")
			p1, _ := term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			fmt.Print("📝 Confirm Passphrase: ")
			p2, _ := term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			if string(p1) == string(p2) && len(p1) > 0 {
				passphrase = string(p1)
				break
			}
			fmt.Println("❌ Passwords do not match. Try again.")
		}
		
		fmt.Println("💾 Creating container file...")
		runCmd("dd", "if=/dev/zero", "of="+VaultPath, "bs=1M", "count="+VaultSizeMB, "status=none")
		
		ensureNodes()
		loopDev := runOutput("sudo", "losetup", "-f", "--show", VaultPath)
		
		fmt.Println("💎 Formatting LUKS...")
		if _, err := runWithStdin(passphrase, "sudo", "cryptsetup", "luksFormat", "--batch-mode", loopDev); err != nil {
			fmt.Println("❌ Format failed.")
			os.Exit(1)
		}
		
		fmt.Println("🔓 Opening vault...")
		runWithStdin(passphrase, "sudo", "cryptsetup", "open", loopDev, MapperName)
		
		exec.Command("sudo", "dmsetup", "mknodes").Run()
		waitForDevice("/dev/mapper/" + MapperName)
		fmt.Println("⛑️  Creating Filesystem...")
		runCmd("sudo", "mkfs.ext4", "-q", "/dev/mapper/"+MapperName)
	} else {
		fmt.Println("🔐 TAZPOD UNLOCK")
		fmt.Print("🔑 Enter Master Passphrase: ")
		p, _ := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		passphrase = string(p)
		
		ensureNodes()
		// Detach stale loops
		exec.Command("bash", "-c", "sudo losetup -a | grep 'vault.img' | cut -d: -f1 | xargs -r sudo losetup -d").Run()
		
		loopDev := runOutput("sudo", "losetup", "-f", "--show", VaultPath)
		if _, err := runWithStdin(passphrase, "sudo", "cryptsetup", "open", loopDev, MapperName); err != nil {
			fmt.Println("❌ DECRYPTION FAILED.")
			runCmd("sudo", "losetup", "-d", loopDev)
			os.Exit(1)
		}
		exec.Command("sudo", "dmsetup", "mknodes").Run()
		waitForDevice("/dev/mapper/" + MapperName)
	}

	os.MkdirAll(MountPath, 0755)
	runCmd("sudo", "mount", "/dev/mapper/"+MapperName, MountPath)
	runCmd("sudo", "chown", "tazpod:tazpod", MountPath)
	fmt.Println("✅ Vault secured and mounted in ~/secrets")
}

func lock() {
	if !isMounted(MountPath) && !fileExist("/dev/mapper/"+MapperName) {
		return
	}
	fmt.Println("🔒 Locking TazPod...")
	exec.Command("sudo", "umount", "-f", MountPath).Run()
	exec.Command("sudo", "cryptsetup", "close", MapperName).Run()
	exec.Command("bash", "-c", "sudo losetup -a | grep 'vault.img' | cut -d: -f1 | xargs -r sudo losetup -d").Run()
	fmt.Println("✅ Vault closed.")
}

func reinit() {
	fmt.Print("⚠️  WARNING: DELETE current vault? (y/N): ")
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" {
		fmt.Println("Aborted.")
		return
	}
	lock()
	fmt.Println("🗑️  Deleting vault file...")
	os.Remove(VaultPath)
	unlock()
}

// --- UTILS ---

func checkInside() {
	if _, err := os.Stat("/.dockerenv"); os.IsNotExist(err) {
		fmt.Println("❌ This command must be run INSIDE the TazPod container.")
		os.Exit(1)
	}
}

func ensureNodes() {
	exec.Command("sudo", "mknod", "/dev/loop-control", "c", "10", "237").Run()
	for i := 0; i < 32; i++ {
		exec.Command("sudo", "mknod", fmt.Sprintf("/dev/loop%d", i), "b", "7", fmt.Sprintf("%d", i)).Run()
	}
}

func runCmd(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.Run()
}

func runOutput(name string, args ...string) string {
	out, _ := exec.Command(name, args...).Output()
	return strings.TrimSpace(string(out))
}

func runWithStdin(input, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = bytes.NewBufferString(input)
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	err := cmd.Run()
	return out.String(), err
}

func isMounted(path string) bool {
	out, _ := exec.Command("mount").Output()
	return strings.Contains(string(out), path)
}

func fileExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func waitForDevice(path string) {
	for i := 0; i < 20; i++ {
		if fileExist(path) { return }
		time.Sleep(200 * time.Millisecond)
	}
}
