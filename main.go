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
		Debug     bool `yaml:"debug"`
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
	} `yaml:"config"`
	Secrets []SecretMapping `yaml:"secrets"`
}

const (
	VaultDir      = "/workspace/.tazpod-vault"
	VaultPath     = VaultDir + "/vault.img"
	MountPath     = "/home/tazpod/secrets"
	MapperName    = "tazpod_vault"
	VaultSizeMB   = "512"
	GhostEnvVar   = "TAZPOD_GHOST_MODE"
	DebugEnvVar   = "TAZPOD_DEBUG"
	TazPodUID     = 1000
	TazPodGID     = 1000
	ConfigPath    = ".tazpod/config.yaml"
	SecretsYAML   = "/workspace/secrets.yml"
	EnvFile       = MountPath + "/.env-infisical"
	
	// FINAL AUTH LOCATION (Inside Vault)
	InfisicalEnclaveDir = MountPath + "/.infisical-vault"
	StayMarker          = "/tmp/.tazpod_stay"
)

var (
	cfg    Config
	secCfg SecretsConfig
)

func main() {
	if len(os.Args) < 2 { help(); os.Exit(1) }
	loadConfigs()
	command := os.Args[1]

	switch command {
	case "up": up()
	case "down": down()
	case "enter", "ssh": enter()
	case "pull", "sync": pull()
	case "login": login()
	case "env": printEnv()
	case "reinit": reinit()
	case "internal-ghost": internalGhost()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}

// --- LOGGING ---

func logDebug(format string, a ...interface{}) {
	if cfg.Features.Debug || os.Getenv(DebugEnvVar) == "true" {
		fmt.Printf("\033[1;30m[DEBUG] "+format+"\033[0m\n", a...)
	}
}

func loadConfigs() {
	cfg.Image = "tazzo/tazlab.net:tazpod-base"
	cfg.ContainerName = "tazpod-lab"; cfg.User = "tazpod"
	if data, err := os.ReadFile(ConfigPath); err == nil { yaml.Unmarshal(data, &cfg) }
	if data, err := os.ReadFile(SecretsYAML); err == nil { yaml.Unmarshal(data, &secCfg) }
}

func help() { fmt.Println("TazPod CLI v8.4 - Native Enclave Persistence") }

// --- INFISICAL RUNNER ---

func runInfisical(args ...string) ([]byte, error) {
	cmd := exec.Command("infisical", args...)
	cmd.Env = append(os.Environ(), 
		"INFISICAL_CONFIG_DIR="+InfisicalEnclaveDir,
		"INFISICAL_VAULT_BACKEND=file",
		"HOME=/home/tazpod",
		"USER=tazpod",
	)
	logDebug("Exec: infisical %s", strings.Join(args, " "))
	return cmd.CombinedOutput()
}

func runInfisicalInteractive(args ...string) error {
	cmd := exec.Command("infisical", args...)
	cmd.Env = append(os.Environ(), 
		"INFISICAL_CONFIG_DIR="+InfisicalEnclaveDir,
		"INFISICAL_VAULT_BACKEND=file",
		"HOME=/home/tazpod",
		"USER=tazpod",
	)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// --- LOGIC ---

func pull() {
	if os.Getenv(GhostEnvVar) != "true" {
		fmt.Println("👻 Vault closed. Starting auto-unlock & pull...")
		cmd := exec.Command("sudo", "unshare", "--mount", "--propagation", "private", "/usr/local/bin/tazpod", "internal-ghost", "pull")
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		cmd.Run()
		return
	}
	internalEnsureAuth()
	syncSecrets()
}

func login() {
	if os.Getenv(GhostEnvVar) != "true" {
		fmt.Println("👻 Vault closed. Opening enclave for login...")
		cmd := exec.Command("sudo", "unshare", "--mount", "--propagation", "private", "/usr/local/bin/tazpod", "internal-ghost", "login")
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		cmd.Run()
		return
	}
	internalLogin()
}

func internalLogin() {
	fmt.Println("🔐 Infisical Login Sequence...")
	os.MkdirAll(InfisicalEnclaveDir, 0755)
	exec.Command("chown", "-R", "tazpod:tazpod", InfisicalEnclaveDir).Run()
	runInfisical("vault", "set", "file")
	runInfisicalInteractive("login")
	runInfisicalInteractive("init")
}

func internalGhost() {
	os.Setenv(GhostEnvVar, "true")
	if cfg.Features.Debug { os.Setenv(DebugEnvVar, "true") }

	requestedCmd := ""
	if len(os.Args) > 2 { requestedCmd = os.Args[2] }

	passphrase := performUnlock()
	mountVault(passphrase)
	
	// AUTO-MIGRATE LEGACY AUTH
	migrateLegacyAuth()
	
os.MkdirAll(InfisicalEnclaveDir, 0755)
	exec.Command("chown", "-R", "tazpod:tazpod", InfisicalEnclaveDir).Run()

	if requestedCmd == "pull" {
		internalEnsureAuth()
		syncSecrets()
	} else if requestedCmd == "login" {
		internalLogin()
	}

	fmt.Println("\n✅ TAZPOD GHOST MODE ACTIVE.")
	bashCmd := exec.Command("bash")
	bashCmd.Stdin, bashCmd.Stdout, bashCmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	bashCmd.SysProcAttr = &syscall.SysProcAttr{ Credential: &syscall.Credential{Uid: uint32(TazPodUID), Gid: uint32(TazPodGID)} }
	
	newEnv := os.Environ()
	newEnv = append(newEnv, 
		GhostEnvVar+"=true", 
		"USER=tazpod", 
		"HOME=/home/tazpod",
		"INFISICAL_CONFIG_DIR="+InfisicalEnclaveDir,
		"INFISICAL_VAULT_BACKEND=file",
	)
	if cfg.Features.Debug { newEnv = append(newEnv, DebugEnvVar+"=true") }
	
	if data, err := os.ReadFile(EnvFile); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "export ") {
				kv := strings.TrimPrefix(line, "export "); parts := strings.SplitN(kv, "=", 2)
				if len(parts) == 2 { newEnv = append(newEnv, fmt.Sprintf("%s=%s", parts[0], strings.Trim(parts[1], "'\""))) }
			}
		}
	}
	bashCmd.Env = newEnv; bashCmd.Run()

	fmt.Println("\n🔒 Locking Ghost Enclave...")
	exec.Command("umount", "-l", MountPath).Run()
	cleanupMappers()
	exec.Command("bash", "-c", "losetup -a | grep 'vault.img' | cut -d: -f1 | xargs -r sudo losetup -d").Run()
}

func migrateLegacyAuth() {
	legacyPaths := []string{
		MountPath + "/.infisical-storage",
		MountPath + "/.infisical-auth",
		MountPath + "/.auth-infisical",
	}
	for _, old := range legacyPaths {
		if _, err := os.Stat(old); err == nil {
			logDebug("Migrating legacy session from %s...", old)
			if _, errDir := os.Stat(InfisicalEnclaveDir); os.IsNotExist(errDir) {
				exec.Command("mv", old, InfisicalEnclaveDir).Run()
			} else {
				exec.Command("rm", "-rf", old).Run()
			}
		}
	}
}

func syncSecrets() {
	fmt.Println("📦 Syncing secrets...")
	pID := secCfg.Config.ProjectID
	args := []string{"export", "--format=dotenv", "--silent"}
	if pID != "" { args = append(args, "--projectId", pID) }
	args = append(args, "--env", "dev")

	out, err := runInfisical(args...)
	if err == nil && len(out) > 0 {
		os.WriteFile(EnvFile, out, 0600); os.Chown(EnvFile, TazPodUID, TazPodGID)
		logDebug("Env file updated in vault.")
	}

	for _, s := range secCfg.Secrets {
		target := filepath.Join(MountPath, s.File)
		fmt.Printf("⬇️  Pulling [%s] -> [%s]... ", s.Name, s.File) 
		cmdArgs := []string{"secrets", "get", s.Name, "--plain"}
		if pID != "" { cmdArgs = append(cmdArgs, "--projectId", pID) }
		cmdArgs = append(cmdArgs, "--env", "dev")

		val, err := runInfisical(cmdArgs...)
		if err == nil && len(strings.TrimSpace(string(val))) > 0 {
			os.WriteFile(target, val, 0600); os.Chown(target, TazPodUID, TazPodGID)
			fmt.Println("✅ OK")
		} else {
			if _, err := os.Stat(target); err == nil { fmt.Println("⚠️  KEEPING LOCAL COPY") } else { fmt.Println("❌ FAILED") }
		}
	}
}

func printEnv() {
	fmt.Printf("export INFISICAL_CONFIG_DIR='%s'\n", InfisicalEnclaveDir)
	fmt.Printf("export INFISICAL_VAULT_BACKEND='file'\n")
	if data, err := os.ReadFile(EnvFile); err == nil { fmt.Print(string(data)) }
	for _, s := range secCfg.Secrets {
		if s.Env != "" {
			target := filepath.Join(MountPath, s.File)
			if _, err := os.Stat(target); err == nil {
				fmt.Printf("export %s='%s'\n", s.Env, target)
			} else { fmt.Printf("unset %s\n", s.Env) }
		}
	}
}

func internalEnsureAuth() {
	configPath := filepath.Join(InfisicalEnclaveDir, "infisical-config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println("🔑 Infisical session missing in enclave."); internalLogin(); return
	}
	pID := secCfg.Config.ProjectID
	args := []string{"secrets", "--env", "dev", "--silent"}
	if pID != "" { args = append(args, "--projectId", pID) }
	if out, err := runInfisical(args...); err != nil {
		logDebug("Auth Test Failed: %s", string(out))
		fmt.Println("🔑 Infisical session invalid or expired."); internalLogin()
	}
}

// --- SYSTEM HELPERS ---

func mountVault(passphrase string) {
	ensureNodes(); cleanupMappers()
	exec.Command("bash", "-c", "losetup -a | grep 'vault.img' | cut -d: -f1 | xargs -r sudo losetup -d").Run()
	isNew := false
	if !fileExist(VaultPath) { isNew = true; runCmd("dd", "if=/dev/zero", "of="+VaultPath, "bs=1M", "count="+VaultSizeMB, "status=none") }
	loopDev := runOutput("losetup", "-f", "--show", VaultPath)
	if isNew { runWithStdin(passphrase, "cryptsetup", "luksFormat", "--batch-mode", "--key-file", "-", loopDev) }
	if _, err := runWithStdin(passphrase, "cryptsetup", "open", "--key-file", "-", loopDev, MapperName); err != nil { os.Exit(1) }
	exec.Command("dmsetup", "mknodes").Run()
	waitForDevice("/dev/mapper/" + MapperName)
	if isNew { runCmd("mkfs.ext4", "-q", "/dev/mapper/"+MapperName) }
	os.MkdirAll(MountPath, 0755)
	exec.Command("mount", "-o", "rw", "-t", "ext4", "/dev/mapper/"+MapperName, MountPath).Run()
	exec.Command("chown", "-R", "tazpod:tazpod", MountPath).Run()
}

func performUnlock() string {
	var passphrase string
	if !fileExist(VaultPath) {
		fmt.Println("🆕 Creating new vault..."); for {
			fmt.Print("📝 Define Passphrase: "); p1, _ := term.ReadPassword(int(syscall.Stdin)); fmt.Println()
			fmt.Print("📝 Confirm: "); p2, _ := term.ReadPassword(int(syscall.Stdin)); fmt.Println()
			if string(p1) == string(p2) && len(p1) > 0 { passphrase = string(p1); break }
		}
	} else { fmt.Print("🔑 Enter Passphrase: "); p, _ := term.ReadPassword(int(syscall.Stdin)); fmt.Println(); passphrase = string(p) }
	return passphrase
}

func cleanupMappers() {
	if exec.Command("dmsetup", "info", MapperName).Run() == nil {
		exec.Command("cryptsetup", "close", MapperName).Run()
		if exec.Command("dmsetup", "info", MapperName).Run() == nil { exec.Command("dmsetup", "remove", "--force", MapperName).Run() }
	}
}

func reinit() {
	if os.Getenv(GhostEnvVar) == "true" { fmt.Println("❌ Exit Ghost Mode first."); os.Exit(1) }
	fmt.Print("⚠️  WIPE VAULT? (y/N): "); var c string; fmt.Scanln(&c)
	if strings.ToLower(c) == "y" { os.Remove(VaultPath); pull() }
}

func runCmd(name string, args ...string) { cmd := exec.Command(name, args...); cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr; cmd.Run() }
func runOutput(name string, args ...string) string { out, _ := exec.Command(name, args...).Output(); return strings.TrimSpace(string(out)) }
func runWithStdin(input, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...); cmd.Stdin = bytes.NewBufferString(input); var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr; err := cmd.Run(); return out.String(), err
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