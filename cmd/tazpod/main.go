package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"tazpod/internal/vault"

	"gopkg.in/yaml.v3"
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
	Version       = "v0.2.0"
	ConfigPath    = ".tazpod/config.yaml"
	SecretsYAML   = "/workspace/secrets.yml"
	EnvFile       = vault.MountPath + "/.env-infisical"
	
	TazPodUID     = 1000
	TazPodGID     = 1000
)

var (
	cfg    Config
	secCfg SecretsConfig
)

func main() {
	if len(os.Args) < 2 { help(); os.Exit(1) }
	arg := os.Args[1]

	if arg == "--version" || arg == "-v" {
		fmt.Printf("🛡️  TazPod %s\n", Version)
		os.Exit(0)
	}

	loadConfigs()
	
	switch arg {
	case "up": up()
	case "down": down()
	case "ssh", "enter": enter()
	case "init": initProject()
	case "unlock": unlock()
	case "lock": vault.Lock()
	case "pull", "sync": pull()
	case "login": login()
	case "save": vault.Save("") 
	case "__internal_env": printExportEnv()
	default: help()
	}
}

func loadConfigs() {
	if data, err := os.ReadFile(ConfigPath); err == nil { yaml.Unmarshal(data, &cfg) }
	if data, err := os.ReadFile(SecretsYAML); err == nil { yaml.Unmarshal(data, &secCfg) }
}

func help() { 
	fmt.Printf("🛡️  TazPod CLI %s (RAM Vault)\n", Version)
}

func up() {
	fmt.Println("🚀 Starting TazPod Container...")
	cwd, _ := os.Getwd()
	cmd := exec.Command("docker", "run", "-d", "--name", cfg.ContainerName, "--privileged", "--network", "host", "-v", cwd+":/workspace", cfg.Image, "sleep", "infinity")
	if out, err := cmd.CombinedOutput(); err != nil { fmt.Printf("❌ Failed: %s\n", string(out)) } else { fmt.Println("✅ Started.") }
}

func down() { exec.Command("docker", "rm", "-f", cfg.ContainerName).Run(); fmt.Println("✅ Stopped.") }

func enter() {
	binary, _ := exec.LookPath("docker")
	// Forziamo la directory di lavoro a /workspace
	args := []string{"docker", "exec", "-it", "-w", "/workspace", cfg.ContainerName, "bash"}
	cmd := exec.Command(binary, args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Run()
	fmt.Println("\n🔒 Session ended. Locking vault...")
	exec.Command("docker", "exec", cfg.ContainerName, "tazpod", "lock").Run()
}

func initProject() { os.Mkdir(".tazpod", 0755); fmt.Println("✅ Project initialized.") }

func unlock() { vault.Unlock() }

func pull() {
	if !isMounted(vault.MountPath) {
		fmt.Println("🔒 Vault locked. Unlocking first...")
		vault.Unlock()
		if !isMounted(vault.MountPath) { return }
	}

	pID := secCfg.Config.ProjectID
	env := secCfg.Config.Env; if env == "" { env = "dev" }
	globalPath := secCfg.Config.Path; if globalPath == "" { globalPath = "/" }

	fmt.Println("📦 Syncing secrets...")
	
	// 1. Prova il sync. Se fallisce per sessione, chiedi login.
	args := []string{"export", "--format=dotenv", "--silent", "--env", env, "--path", globalPath}
	if pID != "" { args = append(args, "--projectId", pID) }
	
	out, stderr, err := runInfisical(args...)
	if err != nil {
		if strings.Contains(stderr, "No valid login session") || strings.Contains(stderr, "login") {
			fmt.Println("👤 Session missing. Logging in...")
			login()
			vault.Save("") // Salva subito il token in RAM -> Disco
			// Riprova il sync
			out, stderr, err = runInfisical(args...)
		}
	}

	if err == nil { 
		os.WriteFile(EnvFile, []byte(out), 0600)
		os.Chown(EnvFile, TazPodUID, TazPodGID)
	} else {
		fmt.Printf("❌ Sync failed: %s\n", stderr)
		return
	}
	
	// 2. Pull individuali
	for _, s := range secCfg.Secrets {
		target := filepath.Join(vault.MountPath, s.File)
		secretPath := s.Path; if secretPath == "" { secretPath = globalPath }
		fmt.Printf("⬇️  %s... ", s.Name)
		cmdArgs := []string{"secrets", "get", s.Name, "--plain", "--env", env, "--path", secretPath}
		if pID != "" { cmdArgs = append(cmdArgs, "--projectId", pID) }
		stdout, _, _ := runInfisical(cmdArgs...)
		if len(strings.TrimSpace(stdout)) > 0 {
			os.WriteFile(target, []byte(strings.TrimSpace(stdout)), 0600)
			os.Chown(target, TazPodUID, TazPodGID)
			fmt.Println("OK")
		} else { fmt.Println("ERR") }
	}
	vault.Save("") 
}

func checkInfisicalLogin() bool {
	domain := secCfg.Config.Domain; if domain == "" { domain = "https://app.infisical.com" }
	stdout, _, err := runInfisical("user", "get", "--domain", domain)
	if err != nil { return false }
	return strings.Contains(stdout, "email") || strings.Contains(stdout, "@")
}

func isMounted(path string) bool {
	data, _ := os.ReadFile("/proc/mounts")
	return strings.Contains(string(data), path)
}

func login() {
	domain := secCfg.Config.Domain; if domain == "" { domain = "https://app.infisical.com" }
	runCmd("infisical", "login", "--domain", domain)
}

func runInfisical(args ...string) (string, string, error) {
	domain := secCfg.Config.Domain; if domain == "" { domain = "https://app.infisical.com" }
	hasDomain := false
	for _, a := range args { if a == "--domain" { hasDomain = true; break } }
	if !hasDomain { args = append(args, "--domain", domain) }

	cmd := exec.Command("infisical", args...)
	cmd.Dir = "/workspace"
	cmd.Env = append(os.Environ(), 
		"INFISICAL_VAULT_BACKEND=file", 
		"INFISICAL_API_URL="+domain,
		"HOME=/home/tazpod", 
		"USER=tazpod")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func runCmd(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = "/workspace"
	if name == "infisical" {
		domain := secCfg.Config.Domain; if domain == "" { domain = "https://app.infisical.com" }
		cmd.Env = append(os.Environ(), 
			"INFISICAL_VAULT_BACKEND=file", 
			"INFISICAL_API_URL="+domain,
			"HOME=/home/tazpod", 
			"USER=tazpod")
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Run()
}

func printExportEnv() {
	// Verifichiamo se la cartella dei segreti esiste ed è leggibile
	// isMounted a volte dà falsi positivi subito dopo un lazy umount
	_, err := os.Stat(vault.PassCache)
	mounted := err == nil 

	for _, s := range secCfg.Secrets {
		if s.Env == "" { continue }
		if mounted {
			target := filepath.Join(vault.MountPath, s.File)
			if _, err := os.Stat(target); err == nil {
				fmt.Printf("export %s=\"%s\"\n", s.Env, target)
			}
		} else {
			fmt.Printf("unset %s\n", s.Env)
		}
	}
}