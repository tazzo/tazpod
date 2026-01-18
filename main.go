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
	StayMarker    = "/tmp/.tazpod_stay"
)

func main() {
	if len(os.Args) < 2 {
		help()
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

func help() {
	fmt.Println("Usage: tazpod <command>")
	fmt.Println("  up      -> Build & Start TazPod container (Host)")
	fmt.Println("  down    -> Stop & Remove TazPod container (Host)")
	fmt.Println("  ssh     -> Enter TazPod container (Host)")
	fmt.Println("  unlock  -> Unlock Vault & Start Secure Shell (Container)")
	fmt.Println("  lock    -> Close Vault & Stay in Container (Container)")
	fmt.Println("  reinit  -> Wipe Vault & Start Fresh (Container)")
}

// --- HOST COMMANDS ---

func up() {
	fmt.Println("🏗️  Ensuring TazPod Engine Image...")
	runCmd("docker", "build", "-f", "Dockerfile.base", "-t", ImageName, ".")
	
	fmt.Println("🛑 Cleaning old instances...")
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
	cmd := exec.Command("sudo", "unshare", "--mount", "--propagation", "private", "/usr/local/bin/tazpod", "internal-ghost")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	
	err := cmd.Run()
	
	// SIGNAL HANDLING: Check if child requested to STAY in container
	if _, statErr := os.Stat(StayMarker); statErr == nil {
		os.Remove(StayMarker)
		os.Exit(2) // Exit code 2 = Stay in shell
	}

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
		}
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

	// Setup Hardware
	exec.Command("mknod", "/dev/loop-control", "c", "10", "237").Run()
	for i := 0; i < 32; i++ { exec.Command("mknod", fmt.Sprintf("/dev/loop%d", i), "b", "7", fmt.Sprintf("%d", i)).Run() }
	os.MkdirAll(VaultDir, 0755)
	
	// CLEANUP ROBUSTO: Interroghiamo il kernel, non il filesystem
	cleanupMappers()
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

	os.MkdirAll(MountPath, 0755)
	runCmd("mount", "-t", "ext4", mapperPath, MountPath)
	runCmd("chown", "tazpod:tazpod", MountPath)

	fmt.Println("\n✅ TAZPOD GHOST MODE ACTIVE.")
	fmt.Println("🚪 Type 'exit' to lock & leave container.")
	fmt.Println("🔒 Type 'tazpod lock' to lock & stay.")
	
	bashCmd := exec.Command("bash")
	bashCmd.Stdin, bashCmd.Stdout, bashCmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	bashCmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uint32(TazPodUID), Gid: uint32(TazPodGID)},
	}
	bashCmd.Env = append(os.Environ(), GhostEnvVar+"=true", "USER=tazpod", "HOME=/home/tazpod")
	bashCmd.Run()

	fmt.Println("\n🔒 Locking Ghost Enclave...")
	exec.Command("umount", "-f", MountPath).Run()
	cleanupMappers()
	exec.Command("bash", "-c", "losetup -a | grep 'vault.img' | cut -d: -f1 | xargs -r losetup -d").Run()
	fmt.Println("✅ Vault locked.")
}

func cleanupMappers() {
	// Chiediamo al kernel: "Esiste tazpod_vault?" (indipendentemente da /dev/mapper/...)
	// dmsetup info ritorna 0 se esiste, 1 se no.
	if exec.Command("dmsetup", "info", MapperName).Run() == nil {
		// Esiste! Proviamo a chiudere gentilmente
		exec.Command("cryptsetup", "close", MapperName).Run()
		
		// Se ancora esiste, usiamo le maniere forti
		if exec.Command("dmsetup", "info", MapperName).Run() == nil {
			exec.Command("dmsetup", "remove", "--force", MapperName).Run()
		}
	}
}

func lock() {
	if os.Getenv(GhostEnvVar) == "true" {
		fmt.Println("🔒 Locking requested (Closing Shell)...")
		os.Create(StayMarker)
		syscall.Kill(os.Getppid(), syscall.SIGKILL)
		return
	}
	fmt.Println("ℹ️  Vault is not mounted (or you are not in Ghost Mode).")
}

func reinit() {
	if os.Getenv(GhostEnvVar) == "true" {
		fmt.Println("❌ Cannot reinit inside Ghost Mode.")
		fmt.Println("🔒 Run 'tazpod lock' first.")
		os.Exit(1)
	}

	fmt.Print("⚠️  DELETE current vault? (y/N): ")
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" { return }
	
	fmt.Println("🗑️  Deleting vault...")
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
	if err != nil {
		fmt.Printf("\n❌ SYSTEM ERROR [%s]: %s\n", name, stderr.String())
	}
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
