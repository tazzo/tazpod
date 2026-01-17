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
		fmt.Println("Usage: tazpod [up|down|unlock|lock|reinit]")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "up":
		up()
	case "down":
		down()
	case "unlock":
		unlock()
	case "lock":
		lock()
	case "reinit":
		reinit()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

// --- HOST COMMANDS ---

func up() {
	fmt.Println("🏗️  Ensuring TazPod Engine (Local)...")
	runCmd("docker", "build", "-f", "Dockerfile.base", "-t", ImageName, ".")
	fmt.Println("🛑 Cleaning instances...")
	exec.Command("docker", "rm", "-f", ContainerName).Run()
	cwd, _ := os.Getwd()
	fmt.Printf("🚀 Starting TazPod in %s...\n", cwd)
	runCmd("docker", "run", "-d", "--name", ContainerName, "--privileged", "--network", "host", "-v", cwd+":/workspace", "-w", "/workspace", ImageName, "sleep", "infinity")
	fmt.Println("✅ Ready. Entry: docker exec -it tazpod-lab bash")
}

func down() {
	fmt.Println("🧹 Shutting down TazPod...")
	runCmd("docker", "rm", "-f", ContainerName)
}

// --- CONTAINER COMMANDS ---

func unlock() {
	if isMounted(MountPath) {
		fmt.Println("✅ Vault already active.")
		return
	}

	// PURGE: Prima di tutto, puliamo il kernel da rimasugli
	purgeStaleMapper()

	var passphrase string
	if !fileExist(VaultPath) {
		fmt.Println("🆕 Vault not found. Creating NEW local vault...")
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
			fmt.Println("❌ Passwords do not match or empty. Try again.")
		}
		
		fmt.Println("💾 Creating container file...")
		runCmd("dd", "if=/dev/zero", "of="+VaultPath, "bs=1M", "count="+VaultSizeMB, "status=none")
		
		ensureNodes()

		loopDev := runOutput("sudo", "losetup", "-f", "--show", VaultPath)
		fmt.Println("💎 Formatting LUKS...")
		if _, err := runWithStdin(passphrase, "sudo", "cryptsetup", "luksFormat", "--batch-mode", loopDev); err != nil {
			fmt.Println("❌ LUKS Format failed.")
			os.Exit(1)
		}
		
		fmt.Println("🔓 Opening vault...")
		if _, err := runWithStdin(passphrase, "sudo", "cryptsetup", "open", loopDev, MapperName); err != nil {
			fmt.Println("❌ LUKS Open failed.")
			os.Exit(1)
		}
		
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
		cleanStaleLoops()

		loopDev := runOutput("sudo", "losetup", "-f", "--show", VaultPath)
		if _, err := runWithStdin(passphrase, "sudo", "cryptsetup", "open", loopDev, MapperName); err != nil {
			fmt.Println("❌ DECRYPTION FAILED.")
			runCmd("sudo", "losetup", "-d", loopDev)
			os.Exit(1)
		}
		waitForDevice("/dev/mapper/" + MapperName)
	}

	os.MkdirAll(MountPath, 0755)
	runCmd("sudo", "mount", "/dev/mapper/"+MapperName, MountPath)
	runCmd("sudo", "chown", "tazpod:tazpod", MountPath)
	fmt.Println("✅ Vault secured and mounted in ~/secrets")
}

func lock() {
	fmt.Println("🔒 Locking TazPod...")
	exec.Command("sudo", "umount", "-f", MountPath).Run()
	exec.Command("sudo", "cryptsetup", "close", MapperName).Run()
	exec.Command("sudo", "dmsetup", "remove", "-f", MapperName).Run()
	cleanStaleLoops()
	fmt.Println("✅ Vault closed.")
}

func reinit() {
	fmt.Print("⚠️  WARNING: This will DELETE all data in the current vault. Are you sure? (y/N): ")
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" {
		fmt.Println("Aborted.")
		return
	}
	lock()
	fmt.Println("🗑️  Deleting old vault file...")
	os.Remove(VaultPath)
	unlock()
}

// --- UTILS ---

func purgeStaleMapper() {
	// Prova a chiudere in ogni modo possibile
	exec.Command("sudo", "cryptsetup", "close", MapperName).Run()
	exec.Command("sudo", "dmsetup", "remove", "-f", MapperName).Run()
}

func ensureNodes() {
	exec.Command("sudo", "mknod", "/dev/loop-control", "c", "10", "237").Run()
	for i := 0; i < 32; i++ {
		exec.Command("sudo", "mknod", fmt.Sprintf("/dev/loop%%d", i), "b", "7", fmt.Sprintf("%%d", i)).Run()
	}
	exec.Command("sudo", "dmsetup", "mknodes").Run()
}

func cleanStaleLoops() {
	exec.Command("bash", "-c", "sudo losetup -a | grep 'vault.img' | cut -d: -f1 | xargs -r sudo losetup -d").Run()
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
	if err != nil {
		fmt.Printf("System error: %s\n", stderr.String())
	}
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
