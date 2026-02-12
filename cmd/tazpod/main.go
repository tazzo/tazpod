package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"tazpod/internal/utils"
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
	Version       = "v0.2.0-beta29"
	ConfigPath    = ".tazpod/config.yaml"
	SecretsYAML   = "/workspace/.tazpod/secrets-sync-config.yml"
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
	case "push": push()
	case "login": login()
	case "save": vault.Save("") 
	case "__internal_env": printExportEnv()
	case "__internal_sync_daemon": syncDaemon()
	default: help()
	}
}

func syncDaemon() {
	for {
		time.Sleep(5 * time.Minute)
		if isVaultUnlocked() {
			pushIdentity()
		}
	}
}

func isVaultUnlocked() bool {
	_, err := os.Stat(vault.PassCache)
	return err == nil
}


func loadConfigs() {
	if data, err := os.ReadFile(ConfigPath); err == nil { yaml.Unmarshal(data, &cfg) }
	// Try loading from new path, fallback to old for migration? 
	// Better just use the constant which now points to the new path.
	if data, err := os.ReadFile(SecretsYAML); err == nil { 
		yaml.Unmarshal(data, &secCfg) 
	} else if data, err := os.ReadFile("/workspace/secrets.yml"); err == nil {
		// Migration fallback
		yaml.Unmarshal(data, &secCfg)
	}
}

func help() { 
	fmt.Printf("🛡️  TazPod CLI %s (RAM Vault)\n", Version)
}

func up() {
	if cfg.ContainerName == "" {
		fmt.Println("❌ Error: container_name not found in .tazpod/config.yaml. Please run 'tazpod init' again.")
		return
	}
	fmt.Println("🚀 Starting TazPod Container...")
	cwd, _ := os.Getwd()
	cmd := exec.Command("docker", "run", "-d", "--name", cfg.ContainerName, "--privileged", "--network", "host", "-v", cwd+":/workspace", cfg.Image, "sleep", "infinity")
	if out, err := cmd.CombinedOutput(); err != nil { 
		fmt.Printf("❌ Failed: %s\n", string(out)) 
	} else { 
		fmt.Println("✅ Started: " + cfg.ContainerName) 
		// Start background sync daemon
		exec.Command("tazpod", "__internal_sync_daemon").Start()
	}
}

func down() { 
	if cfg.ContainerName == "" { return }
	exec.Command("docker", "rm", "-f", cfg.ContainerName).Run()
	fmt.Println("✅ Stopped.") 
}

func enter() {
	if cfg.ContainerName == "" { return }
	binary, _ := exec.LookPath("docker")
	args := []string{"docker", "exec", "-it", "-w", "/workspace", cfg.ContainerName, "bash"}
	cmd := exec.Command(binary, args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Run()
	
	fmt.Println("\n🔒 Session ended. Securing identity...")
	if isVaultUnlocked() {
		pushIdentity()
	}
	exec.Command("docker", "exec", cfg.ContainerName, "tazpod", "lock").Run()
}

func initProject() {
	os.MkdirAll(".tazpod/vault", 0755)
	
	// Generazione Nome Container Unico
	cwd, _ := os.Getwd()
	folderName := filepath.Base(cwd)
	
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomSuffix := fmt.Sprintf("%04d", r.Intn(10000))
	containerName := fmt.Sprintf("tazpod-%s-%s", folderName, randomSuffix)

	// Creazione Config Default
	newCfg := Config{
		Image: "tazzo/tazlab.net:tazpod-gemini",
		ContainerName: containerName,
		User: "tazpod",
	}
	newCfg.Features.GhostMode = true
	newCfg.Features.Debug = false

	data, _ := yaml.Marshal(&newCfg)
	os.WriteFile(ConfigPath, data, 0644)

	// Creazione secrets-sync-config.yml template se non esiste
	newSecretsPath := ".tazpod/secrets-sync-config.yml"
	if _, err := os.Stat(newSecretsPath); os.IsNotExist(err) {
		tmpl := "config:\n  infisical_project_id: \"\"\n  infisical_env: \"dev\"\n  infisical_path: \"/\"\n  infisical_domain: \"https://eu.infisical.com\"\n\nsecrets:\n  - name: EXAMPLE_SECRET\n    file: example-file\n    env: EXAMPLE_ENV\n"
		os.WriteFile(newSecretsPath, []byte(tmpl), 0644)
	}

	fmt.Printf("✅ Project initialized.\n🐳 Container: %s\n", containerName)
}

func unlock() { 
	vault.SetupIdentity()
	vault.Unlock() 
}

func push() {
	subarg := ""
	if len(os.Args) > 2 { subarg = os.Args[2] }

	if subarg == "identity" || subarg == "" {
		pushIdentity()
	} else {
		fmt.Printf("❌ Unknown push target: %s\n", subarg)
	}
}

func pushIdentity() {
	data, err := vault.PackageIdentity()
	if err != nil {
		fmt.Printf("❌ Failed to package identity: %v\n", err)
		return
	}

	// Save temporarily to disk for upload
	tmpFile := "/tmp/tazpod-identity.tar.gz"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		fmt.Printf("❌ Failed to create temp file: %v\n", err)
		return
	}
	defer os.Remove(tmpFile)

	s3, err := utils.NewS3Client("")
	if err != nil {
		fmt.Printf("❌ S3 Client error: %v\n", err)
		return
	}

	fmt.Println("☁️  Uploading identity to S3...")
	if err := s3.UploadFile("identities/default.tar.gz", tmpFile); err != nil {
		fmt.Printf("❌ Upload failed: %v\n", err)
		return
	}
	fmt.Println("✅ Identity pushed successfully.")
}

func pull() {
	subarg := ""
	if len(os.Args) > 2 { subarg = os.Args[2] }

	if subarg == "secrets" {
		pullSecrets()
	} else if subarg == "identity" || subarg == "" {
		pullIdentity()
	} else {
		fmt.Printf("❌ Unknown pull target: %s\n", subarg)
	}
}

func pullIdentity() {
	s3, err := utils.NewS3Client("")
	if err != nil {
		fmt.Printf("❌ S3 Client error: %v\n", err)
		return
	}

	tmpFile := "/tmp/tazpod-identity-pull.tar.gz"
	fmt.Println("☁️  Downloading identity from S3...")
	if err := s3.DownloadFile("identities/default.tar.gz", tmpFile); err != nil {
		fmt.Printf("❌ Download failed: %v\n", err)
		return
	}
	defer os.Remove(tmpFile)

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		fmt.Printf("❌ Failed to read downloaded identity: %v\n", err)
		return
	}

	if err := vault.ExtractIdentity(data); err != nil {
		fmt.Printf("❌ Failed to extract identity: %v\n", err)
		return
	}
	fmt.Println("✅ Identity pulled and extracted.")
}

func pullSecrets() {
	if !isMounted(vault.MountPath) {
		fmt.Println("🔒 Vault locked. Unlocking first...")
		vault.Unlock()
		if !isMounted(vault.MountPath) { return }
	}

	pID := secCfg.Config.ProjectID
	env := secCfg.Config.Env; if env == "" { env = "dev" }
	globalPath := secCfg.Config.Path; if globalPath == "" { globalPath = "/" }

	fmt.Println("📦 Syncing secrets...")
	
	args := []string{"export", "--format=dotenv", "--silent", "--env", env, "--path", globalPath}
	if pID != "" { args = append(args, "--projectId", pID) }
	
	out, stderr, err := runInfisical(args...)
	if err != nil {
		if strings.Contains(stderr, "No valid login session") || strings.Contains(stderr, "login") {
			fmt.Println("👤 Session missing. Logging in...")
			login()
			vault.Save("") 
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
