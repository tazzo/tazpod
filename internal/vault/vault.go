package vault

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"tazpod/internal/utils"

	"golang.org/x/term"
)

const (
	VaultDir    = "/workspace/.tazpod-vault"
	VaultPath   = VaultDir + "/vault.img"
	MountPath   = "/home/tazpod/secrets"
	MapperName  = "tazpod_vault"
	VaultSizeMB = "512"
	GhostEnvVar = "TAZPOD_GHOST_MODE"
	TazPodUID   = 1000
	TazPodGID   = 1000
	StayMarker  = "/tmp/.tazpod_stay"
)

// Unlock handles the Ghost Mode activation or entry
func Unlock() {
	if os.Getenv(GhostEnvVar) == "true" {
		fmt.Println("✅ Already in Ghost Mode.")
		return
	}

	fmt.Println("👻 Entering Ghost Mode (Private Namespace)...")
	cmd := exec.Command("sudo", "unshare", "--mount", "--propagation", "private", "/usr/local/bin/tazpod", "internal-ghost")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	err := cmd.Run()

	if utils.FileExist(StayMarker) {
		os.Remove(StayMarker)
		os.Exit(2)
	}

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
		}
		os.Exit(1)
	}
}

// InternalGhost executes the actual mount and shell spawn inside the namespace
func InternalGhost() {
	if os.Geteuid() != 0 {
		fmt.Println("❌ Error: internal-ghost must run as root.")
		os.Exit(1)
	}

	fmt.Println("🔐 TAZPOD UNLOCK")

	var passphrase string
	if !utils.FileExist(VaultPath) {
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

	ensureNodes()
	os.MkdirAll(VaultDir, 0755)
	CleanupMappers()
	exec.Command("bash", "-c", "losetup -a | grep 'vault.img' | cut -d: -f1 | xargs -r sudo losetup -d").Run()

	mapperPath := "/dev/mapper/" + MapperName

	if !utils.FileExist(VaultPath) {
		fmt.Println("💾 Creating container file...")
		utils.RunCmd("dd", "if=/dev/zero", "of="+VaultPath, "bs=1M", "count="+VaultSizeMB, "status=none")
		loopDev := utils.RunOutput("losetup", "-f", "--show", VaultPath)
		utils.RunWithStdin(passphrase, "cryptsetup", "luksFormat", "--batch-mode", loopDev)
		utils.RunWithStdin(passphrase, "cryptsetup", "open", loopDev, MapperName)
		exec.Command("dmsetup", "mknodes").Run()
		utils.WaitForDevice(mapperPath)
		utils.RunCmd("mkfs.ext4", "-q", mapperPath)
	} else {
		fmt.Println("💾 Unlocking existing vault...")
		loopDev := utils.RunOutput("losetup", "-f", "--show", VaultPath)
		if _, err := utils.RunWithStdin(passphrase, "cryptsetup", "open", loopDev, MapperName); err != nil {
			fmt.Println("❌ DECRYPTION FAILED.")
			utils.RunCmd("losetup", "-d", loopDev)
			os.Exit(1)
		}
		exec.Command("dmsetup", "mknodes").Run()
		utils.WaitForDevice(mapperPath)
	}

	os.MkdirAll(MountPath, 0755)
	utils.RunCmd("mount", "-t", "ext4", mapperPath, MountPath)
	utils.RunCmd("chown", "tazpod:tazpod", MountPath)

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
	utils.RunCmd("umount", "-f", MountPath)
	CleanupMappers()
	exec.Command("bash", "-c", "losetup -a | grep 'vault.img' | cut -d: -f1 | xargs -r sudo losetup -d").Run()
	fmt.Println("✅ Vault locked.")
}

// CleanupMappers safely removes device mapper entries
func CleanupMappers() {
	if exec.Command("dmsetup", "info", MapperName).Run() == nil {
		exec.Command("cryptsetup", "close", MapperName).Run()
		if exec.Command("dmsetup", "info", MapperName).Run() == nil {
			exec.Command("dmsetup", "remove", "--force", MapperName).Run()
		}
	}
}

// Lock handles the request to stay inside the container but close the vault
func Lock() {
	if os.Getenv(GhostEnvVar) == "true" {
		fmt.Println("🔒 Locking requested (Closing Shell)...")
		os.Create(StayMarker)
		syscall.Kill(os.Getppid(), syscall.SIGKILL)
		return
	}
	fmt.Println("ℹ️  Vault is not mounted.")
}

// Reinit wipes the vault
func Reinit() {
	if os.Getenv(GhostEnvVar) == "true" {
		fmt.Println("❌ Cannot reinit inside Ghost Mode.")
		fmt.Println("🔒 Run 'tazpod lock' first.")
		os.Exit(1)
	}
	fmt.Print("⚠️  DELETE current vault? (y/N): ")
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" {
		return
	}
	os.Remove(VaultPath)
	Unlock()
}

func ensureNodes() {
	exec.Command("mknod", "/dev/loop-control", "c", "10", "237").Run()
	for i := 0; i < 64; i++ {
		exec.Command("mknod", fmt.Sprintf("/dev/loop%d", i), "b", "7", fmt.Sprintf("%d", i)).Run()
	}
}
