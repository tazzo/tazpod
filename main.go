package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
	"golang.org/x/term"
)

// --- CONFIGURATION STRUCTS ---

type Config struct {
	Image         string `yaml:"image"`
	ContainerName string `yaml:"container_name"`
	User          string `yaml:"user"`
	Features      struct {
		GhostMode bool `yaml:"ghost_mode"`
	} `yaml:"features"`
}

type SecretMapping struct {
	Name string `yaml:"name"`
	File string `yaml:"file"`
	Env  string `yaml:"env"`
}

type SecretsConfig struct {
	Config struct {
		ProjectID string `yaml:"infisical_project_id"`
		URL       string `yaml:"infisical_url"`
	} `yaml:"config"`
	Secrets []SecretMapping `yaml:"secrets"`
}

var cfg = Config{
	Image:         "tazzo/tazlab.net:tazpod-base",
	ContainerName: "tazpod-lab",
	User:          "tazpod",
}

var secCfg SecretsConfig

const (
	VaultDir      = "/workspace/.tazpod-vault"
	VaultPath     = VaultDir + "/vault.img"
	MountPath     = "/home/tazpod/secrets"
	MapperName    = "tazpod_vault"
	VaultSizeMB   = "512"
	GhostEnvVar   = "TAZPOD_GHOST_MODE"
	TazPodUID     = 1000
	TazPodGID     = 1000
	StayMarker    = "/tmp/.tazpod_stay"
	ConfigPath    = ".tazpod/config.yaml"
	InfisicalDir  = "/home/tazpod/.infisical"
	VaultAuthDir  = MountPath + "/.auth-infisical"
	SecretsYAML   = "/workspace/secrets.yml"
	EnvFile       = MountPath + "/.env-infisical"
)

func main() {
	if len(os.Args) < 2 {
		help()
		os.Exit(1)
	}

	loadAllConfigs()

	command := os.Args[1]

	switch command {
	case "up":
		up()
	case "down":
		down()
	case "enter", "ssh":
		enter()
	case "pull", "sync":
		pull()
	case "login":
		login()
	case "lock":
		lock()
	case "reinit":
		reinit()
	case "internal-ghost":
		internalGhost()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}

// loadAllConfigs reads both TazPod and Project secret configurations
func loadAllConfigs() {
	if data, err := os.ReadFile(ConfigPath); err == nil {
		yaml.Unmarshal(data, &cfg)
	}
	if data, err := os.ReadFile(SecretsYAML); err == nil {
		yaml.Unmarshal(data, &secCfg)
	}
}

func help() {
	fmt.Println("TazPod CLI v5.5 - Zero Trust Dev Environment")
	fmt.Println("Usage: tazpod [up|down|ssh|pull|login|lock|reinit]")
}

// --- CORE LOGIC ---

// pull handles the high-level request to sync secrets, triggering unlock if needed
func pull() {
	if os.Getenv(GhostEnvVar) != "true" {
		fmt.Println("👻 Vault closed. Starting auto-unlock & pull...")
		cmd := exec.Command("sudo", "unshare", "--mount", "--propagation", "private", "/usr/local/bin/tazpod", "internal-ghost", "pull")
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		cmd.Run()
		return
	}
	ensureLogin()
	syncSecrets()
}

// login handles authentication manually, triggering unlock if needed
func login() {
	if os.Getenv(GhostEnvVar) != "true" {
		fmt.Println("👻 Vault closed. Starting auto-unlock & login...")
		cmd := exec.Command("sudo", "unshare", "--mount", "--propagation", "private", "/usr/local/bin/tazpod", "internal-ghost", "login")
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		cmd.Run()
		return
	}
	fmt.Println("🔐 Starting Infisical Login sequence...")
	// Force file backend to avoid keyring errors
	exec.Command("infisical", "vault", "set", "file").Run()
	runCmdInteractive("infisical", "login")
	runCmdInteractive("infisical", "init")
	fixAuthPerms()
	persistAuth()
}

// internalGhost runs as root inside the private mount namespace
func internalGhost() {
	requestedCmd := ""
	if len(os.Args) > 2 { requestedCmd = os.Args[2] }

	// 1. Hardware & Filesystem Unlock
	passphrase := performUnlock()
	mountVault(passphrase)
	
	// 2. Restore Authentication state
	restoreAuth()
	
	// 3. Force file-based vault for the current session
	exec.Command("infisical", "vault", "set", "file").Run()
	fixAuthPerms()

	// 4. Execute requested action before opening shell
	if requestedCmd == "pull" {
		ensureLogin()
		syncSecrets()
	} else if requestedCmd == "login" {
		login()
	}

	// 5. Open the secure shell
	fmt.Println("\n✅ TAZPOD GHOST MODE ACTIVE.")
	bashCmd := exec.Command("bash")
	bashCmd.Stdin, bashCmd.Stdout, bashCmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	
	// Set user identity
	bashCmd.SysProcAttr = &syscall.SysProcAttr{ Credential: &syscall.Credential{Uid: uint32(TazPodUID), Gid: uint32(TazPodGID)} }
	
	// Prepare Environment
	newEnv := os.Environ()
	newEnv = append(newEnv, GhostEnvVar+"=true", "USER=tazpod", "HOME=/home/tazpod")
	
	// Inject secrets from .env-infisical (Cleaned from quotes)
	if data, err := os.ReadFile(EnvFile); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "export ") {
				kv := strings.TrimPrefix(line, "export ")
				parts := strings.SplitN(kv, "=", 2)
				if len(parts) == 2 {
					key := parts[0]
					val := strings.Trim(parts[1], "\"")
					val = strings.Trim(val, "'")
					newEnv = append(newEnv, fmt.Sprintf("%s=%s", key, val))
				}
			}
		}
	}
	bashCmd.Env = newEnv
	bashCmd.Run()

	// 6. Final cleanup and state persistence
	persistAuth()
	fmt.Println("\n🔒 Locking Ghost Enclave...")
	exec.Command("umount", "-f", MountPath).Run()
	cleanupMappers()
	exec.Command("bash", "-c", "losetup -a | grep 'vault.img' | cut -d: -f1 | xargs -r sudo losetup -d").Run()
	fmt.Println("✅ Vault locked.")
}

// syncSecrets pulls individual secrets and generates the environment file
func syncSecrets() {
	fmt.Println("📦 Syncing secrets to ~/secrets/...")
	pID := secCfg.Config.ProjectID
	
	// 1. Update .env-infisical
	args := []string{"export", "--format=dotenv", "--silent"}
	if pID != "" { args = append(args, "--projectId", pID) }
	args = append(args, "--env", "dev")

	out, err := exec.Command("infisical", args...).Output()
	if err == nil {
		os.WriteFile(EnvFile, out, 0600)
		os.Chown(EnvFile, TazPodUID, TazPodGID)
		fmt.Println("✅ Environment file updated.")
	}

	// 2. Pull individual secret files
	for _, s := range secCfg.Secrets {
		target := filepath.Join(MountPath, s.File)
		fmt.Printf("⬇️  Pulling [%s] -> [%s]... ", s.Name, s.File)
		
		cmdArgs := []string{"secrets", "get", s.Name, "--plain"}
		if pID != "" { cmdArgs = append(cmdArgs, "--projectId", pID) }
		cmdArgs = append(cmdArgs, "--env", "dev")

		cmd := exec.Command("infisical", cmdArgs...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		val, err := cmd.Output()
		
		if err == nil && len(val) > 0 {
			os.WriteFile(target, val, 0600)
			os.Chown(target, TazPodUID, TazPodGID)
			fmt.Println("✅ OK")
		} else {
			if _, err := os.Stat(target); err == nil {
				fmt.Println("⚠️  FAILED (Keeping local copy)")
			} else {
				fmt.Println("❌ FAILED (Secret not found in Infisical)")
			}
		}
	}
	fmt.Println("✅ Sync complete.")
}

// ensureLogin checks session and triggers interactive login if required
func ensureLogin() {
	if err := exec.Command("infisical", "whoami").Run(); err != nil {
		fmt.Println("🔑 Infisical session expired or missing. Login required.")
		exec.Command("infisical", "vault", "set", "file").Run()
		runCmdInteractive("infisical", "login")
		if _, err := os.Stat("/workspace/.infisical.json"); os.IsNotExist(err) {
			runCmdInteractive("infisical", "init")
		}
		fixAuthPerms()
		persistAuth()
	}
}

// --- PERSISTENCE HELPERS ---

func persistAuth() {
	if _, err := os.Stat(InfisicalDir); err == nil {
		info, _ := os.Lstat(InfisicalDir)
		if info.Mode()&os.ModeSymlink == 0 {
			os.RemoveAll(VaultAuthDir)
			exec.Command("cp", "-r", InfisicalDir, VaultAuthDir).Run()
			os.RemoveAll(InfisicalDir)
			os.Symlink(VaultAuthDir, InfisicalDir)
			// Chown the actual destination directory in the vault
			exec.Command("chown", "-R", fmt.Sprintf("%d:%d", TazPodUID, TazPodGID), VaultAuthDir).Run()
		}
	}
}

func restoreAuth() {
	if _, err := os.Stat(VaultAuthDir); err == nil {
		os.RemoveAll(InfisicalDir)
		os.Symlink(VaultAuthDir, InfisicalDir)
		exec.Command("chown", "-h", fmt.Sprintf("%d:%d", TazPodUID, TazPodGID), InfisicalDir).Run()
	}
}

func fixAuthPerms() {
	// Ensure the user can always access the auth files
	exec.Command("chown", "-R", fmt.Sprintf("%d:%d", TazPodUID, TazPodGID), InfisicalDir).Run()
	if _, err := os.Stat(VaultAuthDir); err == nil {
		exec.Command("chown", "-R", fmt.Sprintf("%d:%d", TazPodUID, TazPodGID), VaultAuthDir).Run()
	}
}

// --- SYSTEM HELPERS ---

func mountVault(passphrase string) {
	ensureNodes()
	cleanupMappers()
	exec.Command("bash", "-c", "losetup -a | grep 'vault.img' | cut -d: -f1 | xargs -r sudo losetup -d").Run()

	if !fileExist(VaultPath) {
		runCmd("dd", "if=/dev/zero", "of="+VaultPath, "bs=1M", "count="+VaultSizeMB, "status=none")
	}
	loopDev := runOutput("losetup", "-f", "--show", VaultPath)
	if out, _ := exec.Command("cryptsetup", "isLuks", loopDev).CombinedOutput(); !strings.Contains(string(out), "LUKS") {
		runWithStdin(passphrase, "cryptsetup", "luksFormat", "--batch-mode", loopDev)
	}
	if _, err := runWithStdin(passphrase, "cryptsetup", "open", loopDev, MapperName); err != nil {
		fmt.Println("❌ Decrypt Failed."); os.Exit(1)
	}
	exec.Command("dmsetup", "mknodes").Run()
	waitForDevice("/dev/mapper/" + MapperName)
	if out, _ := exec.Command("blkid", "/dev/mapper/"+MapperName).Output(); !strings.Contains(string(out), "ext4") {
		runCmd("mkfs.ext4", "-q", "/dev/mapper/"+MapperName)
	}
	os.MkdirAll(MountPath, 0755)
	runCmd("mount", "-t", "ext4", "/dev/mapper/"+MapperName, MountPath)
	runCmd("chown", fmt.Sprintf("%d:%d", TazPodUID, TazPodGID), MountPath)
}

func performUnlock() string {
	var passphrase string
	if !fileExist(VaultPath) {
		fmt.Println("🆕 Creating new vault...")
		for {
			fmt.Print("📝 Define Passphrase: ")
			p1, _ := term.ReadPassword(int(syscall.Stdin)); fmt.Println()
			fmt.Print("📝 Confirm: ")
			p2, _ := term.ReadPassword(int(syscall.Stdin)); fmt.Println()
			if string(p1) == string(p2) && len(p1) > 0 { passphrase = string(p1); break }
		}
	} else {
		fmt.Print("🔑 Enter Passphrase: ")
		p, _ := term.ReadPassword(int(syscall.Stdin)); fmt.Println()
		passphrase = string(p)
	}
	return passphrase
}

func cleanupMappers() {
	if exec.Command("dmsetup", "info", MapperName).Run() == nil {
		exec.Command("cryptsetup", "close", MapperName).Run()
		if exec.Command("dmsetup", "info", MapperName).Run() == nil {
			exec.Command("dmsetup", "remove", "--force", MapperName).Run()
		}
	}
}

func lock() {
	if os.Getenv(GhostEnvVar) == "true" {
		os.Create(StayMarker)
		syscall.Kill(os.Getppid(), syscall.SIGKILL)
	}
}

func reinit() {
	if os.Getenv(GhostEnvVar) == "true" { fmt.Println("❌ Exit Ghost Mode first."); os.Exit(1) }
	fmt.Print("⚠️  WIPE VAULT? (y/N): "); var c string; fmt.Scanln(&c)
	if strings.ToLower(c) == "y" { os.Remove(VaultPath); pull() }
}

func runCmd(name string, args ...string) { cmd := exec.Command(name, args...); cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr; cmd.Run() }
func runCmdInteractive(name string, args ...string) error { cmd := exec.Command(name, args...); cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr; return cmd.Run() }
func runOutput(name string, args ...string) string { out, _ := exec.Command(name, args...).Output(); return strings.TrimSpace(string(out)) }
func runWithStdin(input, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...); cmd.Stdin = bytes.NewBufferString(input)
	var out, stderr bytes.Buffer; cmd.Stdout, cmd.Stderr = &out, &stderr
	err := cmd.Run(); return out.String(), err
}
func fileExist(path string) bool { _, err := os.Stat(path); return err == nil }
func ensureNodes() {
	exec.Command("sudo", "mknod", "/dev/loop-control", "c", "10", "237").Run()
	for i := 0; i < 64; i++ { exec.Command("sudo", "mknod", fmt.Sprintf("/dev/loop%d", i), "b", "7", fmt.Sprintf("%d", i)).Run() }
}
func waitForDevice(path string) { for i:=0; i<20; i++ { if fileExist(path) { return }; time.Sleep(200*time.Millisecond) } }
func up() {
	display := os.Getenv("DISPLAY"); xauth := os.Getenv("XAUTHORITY")
	if xauth == "" { xauth = os.Getenv("HOME") + "/.Xauthority" }
	cwd, _ := os.Getwd()
	runCmd("docker", "run", "-d", "--name", cfg.ContainerName, "--privileged", "--network", "host", "-e", "DISPLAY="+display, "-e", "XAUTHORITY=/home/tazpod/.Xauthority", "-v", "/tmp/.X11-unix:/tmp/.X11-unix", "-v", xauth+":/home/tazpod/.Xauthority", "-v", cwd+":/workspace", "-w", "/workspace", cfg.Image, "sleep", "infinity")
	fmt.Println("✅ Ready.")
}
func down() { exec.Command("docker", "rm", "-f", cfg.ContainerName).Run() }
func enter() { syscall.Exec("/usr/bin/docker", []string{"docker", "exec", "-it", cfg.ContainerName, "bash"}, os.Environ()) }