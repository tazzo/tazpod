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

type ProviderConfig struct {
	DBHost    string `yaml:"db_host"`
	VPNConfig string `yaml:"vpn_config"`
}

type Config struct {
	Image          string `yaml:"image"`
	ContainerName  string `yaml:"container_name"`
	User           string `yaml:"user"`
	ActiveProvider string `yaml:"active_provider"`
	Providers      map[string]ProviderConfig `yaml:"providers"`
	Features       struct {
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

var (
	Version       = "dev"
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
	case "setup-storage": setupStorage()
	case "vpn": vpnCommand()
	case "memory": memoryCommand()
	default: help()
	}
}

func vpnCommand() {
	subarg := ""
	if len(os.Args) > 2 { subarg = os.Args[2] }
	switch subarg {
	case "up": vpnUp()
	case "down": vpnDown()
	default: fmt.Println("Usage: tazpod vpn [up|down]")
	}
}

func vpnUp() {
	loadEnclaveEnv()
	provider := cfg.ActiveProvider
	if provider == "" { provider = "home" }
	pCfg, ok := cfg.Providers[provider]
	if !ok {
		fmt.Printf("❌ Provider %s not found in config.yaml\n", provider)
		return
	}

	confContent := os.Getenv(pCfg.VPNConfig)
	if confContent == "" {
		fmt.Printf("❌ VPN configuration secret %s not found in environment. Did you run pull secrets?\n", pCfg.VPNConfig)
		return
	}

	// Create temp wg0.conf
	confPath := "/tmp/tazpod-wg0.conf"
	os.WriteFile(confPath, []byte(confContent), 0600)
	defer os.Remove(confPath)

	fmt.Printf("🌐 Bringing up VPN for provider %s...\n", provider)
	// We need sudo for wg-quick. In container it's fine.
	cmd := exec.Command("sudo", "wg-quick", "up", confPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("❌ VPN failed: %s\n", string(out))
	} else {
		fmt.Println("✅ VPN is UP.")
	}
}

func vpnDown() {
	fmt.Println("🌐 Bringing down VPN...")
	// We use the known path or name
	exec.Command("sudo", "wg-quick", "down", "/tmp/tazpod-wg0.conf").Run()
	fmt.Println("✅ VPN is DOWN.")
}

func memoryCommand() {
	subarg := ""
	if len(os.Args) > 2 { subarg = os.Args[2] }
	
	// VPN Auto-Lifecycle
	vpnUp()
	defer vpnDown()

	// Prepare arguments for mnemosyne.py
	pyArgs := []string{"/home/tazpod/memory/mnemosyne.py", subarg}
	if len(os.Args) > 3 {
		pyArgs = append(pyArgs, os.Args[3:]...)
	}

	fmt.Println("🧠 Running Mnemosyne...")
	cmd := exec.Command("python3", pyArgs...)
	cmd.Env = append(os.Environ(), "DB_HOST="+cfg.Providers[cfg.ActiveProvider].DBHost)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Run()
}


func setupStorage() {
	loadEnclaveEnv()
	s3, err := utils.NewS3Client("")
	if err != nil {
		fmt.Printf("❌ S3 Client error: %v\n", err)
		return
	}

	if err := s3.CreateBucket(); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
	} else {
		fmt.Println("✅ Storage setup complete.")
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
		Image: "tazzo/tazpod-gemini:latest",
		ContainerName: containerName,
		User: "tazpod",
		ActiveProvider: "home",
		Providers: map[string]ProviderConfig{
			"home": {
				DBHost: "192.168.1.241",
				VPNConfig: "HOME_WG_CONF",
			},
			"aws": {
				DBHost: "10.0.1.50",
				VPNConfig: "AWS_WG_CONF",
			},
		},
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
	loadEnclaveEnv()
	
	// S3 Authentication Diagnostics
	ak := os.Getenv("AWS_ACCESS_KEY_ID")
	sk := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if ak == "" || sk == "" {
		fmt.Println("⚠️  Warning: AWS credentials missing in environment.")
	} else {
		fmt.Printf("🛡️  Auth Check: ID=%s... (len: %d) | Secret=REDACTED (len: %d)\n", 
			ak[:4], len(ak), len(sk))
	}

	start := time.Now()
	data, err := vault.PackageIdentity()
	if err != nil {
		fmt.Printf("❌ Failed to package identity: %v\n", err)
		return
	}
	fmt.Printf("📦 Packaging completed in %v\n", time.Since(start))

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

	fmt.Println("☁️  Uploading identity to S3 (bucket: tazlab-storage)...")
	uploadStart := time.Now()
	if err := s3.UploadFile("tazpod/identities/global.tar.gz", tmpFile); err != nil {
		fmt.Printf("❌ Upload failed: %v\n", err)
		return
	}
	fmt.Printf("✅ Identity pushed successfully in %v.\n", time.Since(uploadStart))
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
	loadEnclaveEnv()
	s3, err := utils.NewS3Client("")
	if err != nil {
		fmt.Printf("❌ S3 Client error: %v\n", err)
		return
	}

	tmpFile := "/tmp/tazpod-identity-pull.tar.gz"
	fmt.Println("☁️  Downloading identity from S3...")
	if err := s3.DownloadFile("tazpod/identities/global.tar.gz", tmpFile); err != nil {
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

func loadEnclaveEnv() {
	if data, err := os.ReadFile(EnvFile); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") { continue }
			if strings.Contains(line, "=") {
				parts := strings.SplitN(line, "=", 2)
				key := strings.TrimSpace(parts[0])
				// Clean both single and double quotes
				val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				os.Setenv(key, val)
			}
		}
	}
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

	// Identify all unique paths to sync
	paths := make(map[string]bool)
	paths[globalPath] = true
	for _, s := range secCfg.Secrets {
		if s.Path != "" {
			paths[s.Path] = true
		}
	}

	secretsMap := make(map[string]string)
	fmt.Println("📦 Syncing secrets from Infisical...")

	for path := range paths {
		fmt.Printf("  -> Path: %s... ", path)
		args := []string{"export", "--format=dotenv", "--silent", "--env", env, "--path", path}
		if pID != "" { args = append(args, "--projectId", pID) }
		
		out, stderr, err := runInfisical(args...)
		if err != nil {
			if strings.Contains(stderr, "No valid login session") || strings.Contains(stderr, "login") {
				fmt.Println("\n👤 Session missing. Logging in...")
				login()
				vault.Save("") 
				out, stderr, err = runInfisical(args...)
			}
		}

		if err != nil {
			fmt.Printf("ERR (%s)\n", stderr)
			continue
		}

		// Parse and merge
		lines := strings.Split(out, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") { continue }
			if strings.Contains(line, "=") {
				parts := strings.SplitN(line, "=", 2)
				key := strings.TrimSpace(parts[0])
				val := strings.Trim(strings.TrimSpace(parts[1]), "\"")
				secretsMap[key] = val
			}
		}
		fmt.Println("OK")
	}

	// 1. Save main .env-infisical (merged)
	var envBuf bytes.Buffer
	for k, v := range secretsMap {
		envBuf.WriteString(fmt.Sprintf("%s=\"%s\"\n", k, v))
	}
	os.WriteFile(EnvFile, envBuf.Bytes(), 0600)
	os.Chown(EnvFile, TazPodUID, TazPodGID)

	// 2. Write individual files
	fmt.Println("📂 Extracting secret files...")
	for _, s := range secCfg.Secrets {
		target := filepath.Join(vault.MountPath, s.File)
		fmt.Printf("  -> %s... ", s.Name)
		if val, ok := secretsMap[s.Name]; ok {
			os.WriteFile(target, []byte(val), 0600)
			os.Chown(target, TazPodUID, TazPodGID)
			fmt.Println("OK")
		} else {
			fmt.Println("MISSING")
		}
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
