package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

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
	Path string `yaml:"path"`
}

type SecretsConfig struct {
	Config struct {
		ProjectID string `yaml:"infisical_project_id"`
		Env       string `yaml:"infisical_env"`
		Path      string `yaml:"infisical_path"`
		Domain    string `yaml:"infisical_domain"`
	} `yaml:"config"`
	Secrets []SecretMapping `yaml:"secrets"`
}

const (
	VaultDir      = "/workspace/.tazpod/vault" 
	VaultPath     = VaultDir + "/vault.img"
	MountPath     = "/home/tazpod/secrets"
	MapperName    = "tazpod_vault"
	VaultSizeMB   = "512"
	GhostEnvVar   = "TAZPOD_GHOST_MODE"
	TazPodUID     = 1000
	TazPodGID     = 1000
	ConfigPath    = ".tazpod/config.yaml"
	SecretsYAML   = "/workspace/secrets.yml"
	EnvFile       = MountPath + "/.env-infisical"
	
	InfisicalLocalHome    = "/home/tazpod/.infisical"
	InfisicalKeyringLocal = "/home/tazpod/infisical-keyring"
	GeminiLocalHome       = "/home/tazpod/.gemini"
	InfisicalVaultDir     = MountPath + "/.infisical-vault"
	InfisicalKeyringVault = MountPath + "/.infisical-keyring"
	GeminiVaultDir        = MountPath + "/.gemini-vault"
)

var (
	cfg    Config
	secCfg SecretsConfig
)

func main() {
	if len(os.Args) < 2 { help(); os.Exit(1) }
	arg := os.Args[1]
	loadConfigs()
	switch arg {
	case "up": up()
	case "down": down()
	case "ssh": enter()
	case "pull", "sync": pull()
	case "login": login()
	case "init": initProject()
	case "unlock": unlock()
	case "internal-ghost": internalGhost()
	default: os.Exit(1)
	}
}

func loadConfigs() {
	if data, err := os.ReadFile(ConfigPath); err == nil { yaml.Unmarshal(data, &cfg) }
	if data, err := os.ReadFile(SecretsYAML); err == nil { yaml.Unmarshal(data, &secCfg) }
}

func help() { fmt.Printf("🛡️  TazPod CLI v0.1.14 (Ghost Protocol)\n") }

func runInfisicalDebug(args ...string) (string, string, error) {
	var cmd *exec.Cmd
	domain := secCfg.Config.Domain; if domain == "" { domain = "https://app.infisical.com" }
	args = append(args, "--domain", domain)

	if os.Geteuid() == 0 {
		fullArgs := append([]string{"-u", "tazpod", "infisical"}, args...)
		cmd = exec.Command("sudo", fullArgs...)
		cmd.Env = append(os.Environ(), "HOME=/home/tazpod", "USER=tazpod", "INFISICAL_VAULT_BACKEND=file")
	} else {
		cmd = exec.Command("infisical", args...)
		cmd.Env = append(os.Environ(), "INFISICAL_VAULT_BACKEND=file")
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func syncSecrets() {
	fmt.Println("📦 Syncing secrets (v0.1.14)...")
	pID := secCfg.Config.ProjectID
	env := secCfg.Config.Env; if env == "" { env = "dev" }
	globalPath := secCfg.Config.Path; if globalPath == "" { globalPath = "/" }

	// 1. Export env file
	args := []string{"export", "--format=dotenv", "--silent", "--env", env, "--path", globalPath}
	if pID != "" { args = append(args, "--projectId", pID) }
	out, _, err := runInfisicalDebug(args...)
	if err == nil { os.WriteFile(EnvFile, []byte(out), 0600); os.Chown(EnvFile, TazPodUID, TazPodGID) }
	
	// 2. Pull individual secret files
	for _, s := range secCfg.Secrets {
		target := filepath.Join(MountPath, s.File)
		secretPath := s.Path; if secretPath == "" { secretPath = globalPath }
		
		fmt.Printf("⬇️  Pulling [%s] from [%s] -> [%s]... ", s.Name, secretPath, s.File)
		cmdArgs := []string{"secrets", "get", s.Name, "--plain", "--env", env, "--path", secretPath}
		if pID != "" { cmdArgs = append(cmdArgs, "--projectId", pID) }
		
		stdout, stderr, err := runInfisicalDebug(cmdArgs...)
		cleanVal := strings.TrimSpace(stdout)
		
		if err == nil && len(cleanVal) > 0 {
			os.WriteFile(target, []byte(cleanVal), 0600)
			os.Chown(target, TazPodUID, TazPodGID)
			fmt.Println("✅ OK")
		} else {
			fmt.Println("❌ FAILED")
			if strings.Contains(stderr, "No valid login session found") {
				fmt.Println("\n🔒 Session expired. Please run 'tazpod login' inside the vault.")
			} else {
				fmt.Printf("\n   [DEBUG] Error: %v\n   [DEBUG] Stderr: %q\n", err, strings.TrimSpace(stderr))
			}
		}
	}
}

func pull() {
	if os.Getenv(GhostEnvVar) != "true" {
		exe, _ := os.Executable()
		cmd := exec.Command("sudo", "unshare", "--mount", "--propagation", "private", exe, "internal-ghost", "pull")
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr; cmd.Run(); return
	}
	syncSecrets()
}

func initProject() { os.Mkdir(".tazpod", 0755) }
func unlock() {
	exe, _ := os.Executable()
	cmd := exec.Command("sudo", "unshare", "--mount", "--propagation", "private", exe, "internal-ghost")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr; cmd.Run()
}
func login() { 
	if os.Getenv(GhostEnvVar) != "true" {
		exe, _ := os.Executable()
		cmd := exec.Command("sudo", "unshare", "--mount", "--propagation", "private", exe, "internal-ghost", "login")
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr; cmd.Run(); return
	}
	runCmd("infisical", "login") 
}

func internalGhost() {
	os.Setenv(GhostEnvVar, "true")
	requestedCmd := ""
	if len(os.Args) > 2 { requestedCmd = os.Args[2] }
	passphrase := performUnlock()
	mountVault(passphrase); setupBindAuth()

	switch requestedCmd {
	case "pull":
		syncSecrets()
		// Continue to shell...
	case "login":
		runCmd("infisical", "login")
		// Continue to shell...
	}
	
	// Default: Interactive Shell
	bashCmd := exec.Command("bash")
	bashCmd.Stdin, bashCmd.Stdout, bashCmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	bashCmd.SysProcAttr = &syscall.SysProcAttr{ Credential: &syscall.Credential{Uid: uint32(TazPodUID), Gid: uint32(TazPodGID)} }
	newEnv := os.Environ()
	newEnv = append(newEnv, GhostEnvVar+"=true", "USER=tazpod", "HOME=/home/tazpod")
	for _, s := range secCfg.Secrets {
		if s.Env != "" {
			target := filepath.Join(MountPath, s.File)
			if _, err := os.Stat(target); err == nil { newEnv = append(newEnv, fmt.Sprintf("%s=%s", s.Env, target)) }
		}
	}
	bashCmd.Env = newEnv; bashCmd.Run()
}

func setupBindAuth() {
	bridge(InfisicalLocalHome, InfisicalVaultDir)
	bridge(InfisicalKeyringLocal, InfisicalKeyringVault)
	bridge(GeminiLocalHome, GeminiVaultDir)
}

func bridge(local, vault string) {
	// Ensure both source and target exist
	os.MkdirAll(vault, 0755)
	os.MkdirAll(local, 0755)
	
	// Only mount if not already mounted
	if !isMounted(local) {
		fmt.Printf("[DEBUG] Binding %s -> %s\n", vault, local)
		if err := exec.Command("mount", "--bind", vault, local).Run(); err != nil {
			fmt.Printf("❌ Failed to bind mount %s: %v\n", local, err)
		}
	}
}

func mountVault(passphrase string) {
	if isMounted(MountPath) { return }
	
	// Check if mapper already exists
	if _, err := os.Stat("/dev/mapper/" + MapperName); err == nil {
		fmt.Println("[DEBUG] Vault mapper already exists, skipping cryptsetup.")
	} else {
		loopDev := strings.TrimSpace(runOutput("losetup", "-f", "--show", VaultPath))
		if loopDev == "" {
			fmt.Println("❌ Failed to create loop device")
			return
		}
		fmt.Printf("[DEBUG] Using loop device: %s\n", loopDev)
		
		// Open LUKS
		cmd := exec.Command("cryptsetup", "luksOpen", loopDev, MapperName)
		cmd.Stdin = bytes.NewBufferString(passphrase)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("❌ cryptsetup failed: %v\n", err)
			fmt.Printf("   Stderr: %s\n", stderr.String())
			exec.Command("losetup", "-d", loopDev).Run()
			return
		}
	}

	os.MkdirAll(MountPath, 0755)
	
	// Mount
	var stderr bytes.Buffer
	mCmd := exec.Command("mount", "/dev/mapper/"+MapperName, MountPath)
	mCmd.Stderr = &stderr
	if err := mCmd.Run(); err != nil {
		if !strings.Contains(stderr.String(), "already mounted") {
			fmt.Printf("❌ mount failed: %v\n", err)
			fmt.Printf("   Stderr: %s\n", stderr.String())
		}
	}

	exec.Command("chown", "-R", "tazpod:tazpod", MountPath).Run()
}

func performUnlock() string {
	if isMounted(MountPath) { return "" }
	fmt.Print("🔑 Passphrase: "); p, _ := term.ReadPassword(int(syscall.Stdin)); fmt.Println(); return string(p)
}
func isMounted(path string) bool { data, _ := os.ReadFile("/proc/mounts"); return strings.Contains(string(data), path) }
func runOutput(name string, args ...string) string { out, _ := exec.Command(name, args...).Output(); return string(out) }
func runWithStdin(input, name string, args ...string) {
	cmd := exec.Command(name, args...); cmd.Stdin = bytes.NewBufferString(input); cmd.Run()
}
func up() { runCmd("docker", "run", "-d", "--name", cfg.ContainerName, "--privileged", "--network", "host", "-v", "/workspace:/workspace", cfg.Image, "sleep", "infinity") }
func down() { exec.Command("docker", "rm", "-f", cfg.ContainerName).Run() }
func enter() { syscall.Exec("/usr/bin/docker", []string{"docker", "exec", "-it", cfg.ContainerName, "bash"}, os.Environ()) }
func runCmd(name string, args ...string) {
	var cmd *exec.Cmd
	// Use sudo -u tazpod for interactive commands if running as root
	if os.Geteuid() == 0 && name != "docker" && name != "mount" && name != "umount" && name != "cryptsetup" && name != "losetup" {
		fullArgs := append([]string{"-u", "tazpod", name}, args...)
		cmd = exec.Command("sudo", fullArgs...)
		cmd.Env = append(os.Environ(), "HOME=/home/tazpod", "USER=tazpod", "INFISICAL_VAULT_BACKEND=file")
	} else {
		cmd = exec.Command(name, args...)
		if name == "infisical" {
			cmd.Env = append(os.Environ(), "INFISICAL_VAULT_BACKEND=file")
		}
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Run()
}
