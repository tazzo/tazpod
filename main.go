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
	GhostEnvVar   = "TAZPOD_GHOST_MODE"
	TazPodUID     = 1000
	TazPodGID     = 1000
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: tazpod [up|down|enter|ssh|unlock|lock|reinit|internal-ghost]")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "up":
		up()
	case "down":
		down()
	case "enter", "ssh":
		enter()
	case "unlock":
		unlock()
	case "lock":
		lock()
	case "reinit":
		reinit()
	case "internal-ghost":
		internalGhost()
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
	fmt.Println("✅ Ready. Run './tazpod enter' to get inside.")
}

func down() {
	fmt.Println("🧹 Shutting down TazPod...")
	runCmd("docker", "rm", "-f", ContainerName)
	fmt.Println("✅ Done.")
}

func enter() {
	binary, _ := exec.LookPath("docker")
	args := []string{"docker", "exec", "-it", ContainerName, "bash"}
	syscall.Exec(binary, args, os.Environ())
}

// --- CONTAINER COMMANDS ---

func unlock() {
	if os.Getenv(GhostEnvVar) == "true" {
		fmt.Println("✅ Already in Ghost Mode.")
		return
	}

	fmt.Println("👻 Entering Ghost Mode (Private Namespace)...")
	// Creiamo il namespace di mount privato ed eseguiamo internal-ghost come root
	cmd := exec.Command("sudo", "unshare", "--mount", "--propagation", "private", "/usr/local/bin/tazpod", "internal-ghost")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

func internalGhost() {
	if os.Geteuid() != 0 {
		fmt.Println("❌ Error: internal-ghost must run as root.")
		os.Exit(1)
	}

	fmt.Println("🔐 TAZPOD UNLOCK")

	var passphrase string
	if !fileExist(VaultPath) {
		fmt.Println("🆕 Vault not found. Creating NEW local vault...")
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
			fmt.Println("❌ Passwords do not match.")
		}
	} else {
		fmt.Print("🔑 Enter Master Passphrase: ")
		p, _ := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		passphrase = string(p)
	}

	// Setup Hardware Nodes
	exec.Command("mknod", "/dev/loop-control", "c", "10", "237").Run()
	for i := 0; i < 64; i++ { exec.Command("mknod", fmt.Sprintf("/dev/loop%d", i), "b", "7", fmt.Sprintf("%d", i)).Run() }
	os.MkdirAll(VaultDir, 0755)
	
	// Cleanup preventivo intelligente
	mapperDev := "/dev/mapper/" + MapperName
	if fileExist(mapperDev) {
		exec.Command("cryptsetup", "close", MapperName).Run()
	}
	if fileExist(mapperDev) {
		exec.Command("dmsetup", "remove", "-f", MapperName).Run()
	}
	exec.Command("bash", "-c", "losetup -a | grep 'vault.img' | cut -d: -f1 | xargs -r losetup -d").Run()

	mapperPath := "/dev/mapper/" + MapperName

	if !fileExist(VaultPath) {
		fmt.Println("💾 Creating container file...")
		runCmd("dd", "if=/dev/zero", "of="+VaultPath, "bs=1M", "count="+VaultSizeMB, "status=none")
		loopDev := runOutput("losetup", "-f", "--show", VaultPath)
		runWithStdin(passphrase, "cryptsetup", "luksFormat", "--batch-mode", loopDev)
		runWithStdin(passphrase, "cryptsetup", "open", loopDev, MapperName)
		exec.Command("dmsetup", "mknodes").Run()
		waitForDevice(mapperPath)
		runCmd("mkfs.ext4", "-q", mapperPath)
	} else {
		fmt.Println("💾 Unlocking existing vault...")
		loopDev := runOutput("losetup", "-f", "--show", VaultPath)
		if _, err := runWithStdin(passphrase, "cryptsetup", "open", loopDev, MapperName); err != nil {
			fmt.Println("❌ DECRYPTION FAILED.")
			runCmd("losetup", "-d", loopDev); os.Exit(1)
		}
		exec.Command("dmsetup", "mknodes").Run()
		waitForDevice(mapperPath)
	}

	// PRIVATE MOUNT
	os.MkdirAll(MountPath, 0755)
	runCmd("mount", "-t", "ext4", mapperPath, MountPath)
	runCmd("chown", "tazpod:tazpod", MountPath)

	// SPAWN PROTECTED USER SHELL
	fmt.Println("\n✅ TAZPOD GHOST MODE ACTIVE.")
	fmt.Println("🚪 Type 'exit' to lock and leave.")
	
bashCmd := exec.Command("bash")
bashCmd.Stdin, bashCmd.Stdout, bashCmd.Stderr = os.Stdin, os.Stdout, os.Stderr
bashCmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uint32(TazPodUID), Gid: uint32(TazPodGID)},
	}
bashCmd.Env = append(os.Environ(), GhostEnvVar+"=true", "USER=tazpod", "HOME=/home/tazpod")
bashCmd.Run()

	// AUTO-CLEANUP ON EXIT (SMART)
	fmt.Println("\n🔒 Locking and destroying Ghost Enclave...")
	exec.Command("umount", "-f", MountPath).Run()
	
	// Close LUKS
	if fileExist(mapperPath) {
		exec.Command("cryptsetup", "close", MapperName).Run()
	}
	// Force remove only if still there
	if fileExist(mapperPath) {
		exec.Command("dmsetup", "remove", "-f", MapperName).Run()
	}
	
	exec.Command("bash", "-c", "losetup -a | grep 'vault.img' | cut -d: -f1 | xargs -r losetup -d").Run()
	fmt.Println("✅ Vault locked.")
}

func lock() {
	fmt.Println("ℹ️  In Ghost Mode, just type 'exit' to lock.")
}

func reinit() {
	fmt.Print("⚠️  DELETE current vault? (y/N): ")
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" { return }
	os.Remove(VaultPath)
	unlock()
}

// --- UTILS ---

func runCmd(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.Run()
}

func runOutput(name string, args ...string) string {
	out, err := exec.Command(name, args...).Output()
	if err != nil { return "" }
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