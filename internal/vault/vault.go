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
	MountPath     = "/home/tazpod/secrets"
	MapperName    = "tazpod_vault"
	VaultSizeMB   = "512"
	GhostEnvVar   = "TAZPOD_GHOST_MODE"
	TazPodUID     = 1000
	TazPodGID     = 1000
	StayMarker    = "/tmp/.tazpod_stay"
	SecretsYAML   = "/workspace/secrets.yml"
)

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

func InternalGhost() {
	if os.Geteuid() != 0 {
		fmt.Println("❌ Error: internal-ghost must run as root.")
		os.Exit(1)
	}
	fmt.Println("🔐 TAZPOD UNLOCK")
	var passphrase string
	if !utils.FileExist(VaultPath) {
		fmt.Println("🆕 Creating NEW local vault...")
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
		utils.RunCmd("dd", "if=/dev/zero", "of="+VaultPath, "bs=1M", "count="+VaultSizeMB, "status=none")
		loopDev := utils.RunOutput("losetup", "-f", "--show", VaultPath)
		utils.RunWithStdin(passphrase, "cryptsetup", "luksFormat", "--batch-mode", loopDev)
		utils.RunWithStdin(passphrase, "cryptsetup", "open", loopDev, MapperName)
		exec.Command("dmsetup", "mknodes").Run()
		utils.WaitForDevice(mapperPath)
		utils.RunCmd("mkfs.ext4", "-q", mapperPath)
	} else {
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
	bashCmd := exec.Command("bash")
	bashCmd.Stdin, bashCmd.Stdout, bashCmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	bashCmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uint32(TazPodUID), Gid: uint32(TazPodGID)},
	}
	newEnv := os.Environ()
	newEnv = append(newEnv, GhostEnvVar+"=true", "USER=tazpod", "HOME=/home/tazpod")
	
	// Sincronizziamo senza log per la shell (i log sono gestiti da getSecretEnvs)
	envs := getSecretEnvs(true) 
	for k, v := range envs {
		newEnv = append(newEnv, k+"="+v)
	}
	bashCmd.Env = newEnv
	bashCmd.Run()
	fmt.Println("\n🔒 Locking Ghost Enclave...")
	utils.RunCmd("umount", "-f", MountPath)
	CleanupMappers()
	exec.Command("bash", "-c", "losetup -a | grep 'vault.img' | cut -d: -f1 | xargs -r sudo losetup -d").Run()
	fmt.Println("✅ Vault locked.")
}

func getSecretEnvs(showLog bool) map[string]string {
	envs := make(map[string]string)
	if !utils.FileExist(SecretsYAML) {
		if showLog { fmt.Fprintln(os.Stderr, "⚠️  secrets.yml not found") }
		return envs
	}
	countStr := utils.RunOutput("yq", ".secrets | length", SecretsYAML)
	var count int
	fmt.Sscanf(countStr, "%d", &count)
	if showLog { fmt.Fprintln(os.Stderr, "📦 Sourcing secrets from vault...") }
	for i := 0; i < count; i++ {
		fileName := cleanStr(utils.RunOutput("yq", fmt.Sprintf(".secrets[%d].file", i), SecretsYAML))
		envVar := cleanStr(utils.RunOutput("yq", fmt.Sprintf(".secrets[%d].env", i), SecretsYAML))
		if fileName == "" || envVar == "" { continue }
		fullPath := MountPath + "/" + fileName
		if utils.FileExist(fullPath) {
			envs[envVar] = fullPath
			if showLog { fmt.Fprintf(os.Stderr, "  ✅ %s -> $%s\n", fileName, envVar) }
		} else {
			if showLog { fmt.Fprintf(os.Stderr, "  ❌ %s (NOT FOUND)\n", fileName) }
		}
	}
	return envs
}

func ExportEnv() {
	envs := getSecretEnvs(true)
	for k, v := range envs {
		// Concatenazione pura: zero Printf, zero errori
		os.Stdout.WriteString("export " + k + "=\"" + v + "\"\n")
	}
}

func cleanStr(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "`")
	s = strings.Trim(s, "\"")
	s = strings.Trim(s, "'")
	if s == "null" { return "" }
	return s
}

func CleanupMappers() {
	if exec.Command("dmsetup", "info", MapperName).Run() == nil {
		exec.Command("cryptsetup", "close", MapperName).Run()
		if exec.Command("dmsetup", "info", MapperName).Run() == nil {
			exec.Command("dmsetup", "remove", "--force", MapperName).Run()
		}
	}
}

func Lock() {
	if os.Getenv(GhostEnvVar) == "true" {
		fmt.Println("🔒 Locking requested...")
		os.Create(StayMarker)
		syscall.Kill(os.Getppid(), syscall.SIGKILL)
		return
	}
}

func Reinit() {
	if os.Getenv(GhostEnvVar) == "true" {
		fmt.Println("❌ Cannot reinit inside Ghost Mode. Run 'tazpod lock' first.")
		os.Exit(1)
	}
	fmt.Print("⚠️  DELETE current vault? (y/N): ")
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" { return }
	os.Remove(VaultPath)
	Unlock()
}

func ensureNodes() {
	exec.Command("sudo", "mknod", "/dev/loop-control", "c", "10", "237").Run()
	for i := 0; i < 64; i++ {
		exec.Command("sudo", "mknod", fmt.Sprintf("/dev/loop%%d", i), "b", "7", fmt.Sprintf("%d", i)).Run()
	}
}