package main

import (
	"bufio"
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

type AwsSsoConfig struct {
	StartURL  string `yaml:"start_url"`
	AccountID string `yaml:"account_id"`
	RoleName  string `yaml:"role_name"`
	Region    string `yaml:"region"`
	Profile   string `yaml:"profile"`
}

type Config struct {
	Image          string `yaml:"image"`
	ContainerName  string `yaml:"container_name"`
	User           string `yaml:"user"`
	ActiveProvider string `yaml:"active_provider"`
	Providers      map[string]ProviderConfig `yaml:"providers"`
	AwsSso         AwsSsoConfig `yaml:"aws_sso"`
	Features       struct {
		GhostMode bool `yaml:"ghost_mode"`
		Debug     bool `yaml:"debug"`
	} `yaml:"features"`
}

var (
	Version       = "dev"
	ConfigPath    = ".tazpod/config.yaml"
	
	TazPodUID     = 1000
	TazPodGID     = 1000
)

var cfg Config

func main() {
	if len(os.Args) < 2 { smartEntry(); return }
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
	if err := os.WriteFile(confPath, []byte(confContent), 0600); err != nil {
		fmt.Printf("❌ Failed to write VPN config: %v\n", err)
		return
	}
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


func setupStorage() {
	s3, err := utils.NewS3Client("", cfg.AwsSso.Profile)
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
			vault.Save("")
			pushVault()
		}
	}
}

func isVaultUnlocked() bool {
	_, err := os.Stat(vault.PassCache)
	return err == nil
}


func loadConfigs() {
	data, err := os.ReadFile(ConfigPath)
	if os.IsNotExist(err) {
		return // no config yet, normal on first run
	}
	if err != nil {
		fmt.Printf("⚠️  Could not read %s: %v\n", ConfigPath, err)
		return
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		fmt.Printf("❌ Invalid config %s: %v\n", ConfigPath, err)
		os.Exit(1)
	}
}

func help() { 
	fmt.Printf("🛡️  TazPod CLI %s (RAM Vault)\n", Version)
	fmt.Println("\nUsage: tazpod <command> [arguments]")
	fmt.Println("\nLifecycle Commands:")
	fmt.Println("  init           Initialize a new TazPod project in the current directory")
	fmt.Println("  up             Start the development container")
	fmt.Println("  down           Stop and remove the development container")
	fmt.Println("  ssh | enter    Enter the container shell")
	fmt.Println("\nVault & Secrets Commands:")
	fmt.Println("  unlock         Unlock the RAM vault (Ghost Mode)")
	fmt.Println("  lock           Lock and wipe the RAM vault")
	fmt.Println("  save           Save the current RAM vault content to disk")
	fmt.Println("  login          Authenticate with AWS SSO")
	fmt.Println("  pull [vault|identity] Pull vault or identity from S3 (default: vault, alias: sync)")
	fmt.Println("  push [vault|identity] Push vault or identity to S3 (default: vault)")
	fmt.Println("\nUtility Commands:")
	fmt.Println("  vpn up|down    Manage VPN connection for the active provider")
	fmt.Println("  setup-storage  Initialize S3 bucket for nomadic identity")
	fmt.Println("  --version, -v  Show version information")
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
		vault.Save("")
		pushVault()
	}
	exec.Command("docker", "exec", cfg.ContainerName, "tazpod", "lock").Run()
}

func initProject() {
	os.MkdirAll(".tazpod/vault", 0755)

	// Generate unique container name (always fresh, never from template)
	cwd, _ := os.Getwd()
	folderName := filepath.Base(cwd)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomSuffix := fmt.Sprintf("%04d", r.Intn(10000))
	containerName := fmt.Sprintf("tazpod-%s-%s", folderName, randomSuffix)

	var newCfg Config

	// Try to load defaults from ~/secrets/tazpod-template.yaml
	templatePath := filepath.Join(os.Getenv("HOME"), "secrets", "tazpod-template.yaml")
	if data, err := os.ReadFile(templatePath); err == nil {
		if err := yaml.Unmarshal(data, &newCfg); err == nil {
			fmt.Printf("📋 Template loaded from %s\n", templatePath)
		} else {
			fmt.Printf("⚠️  Template found but invalid YAML (%v) — switching to interactive.\n", err)
			newCfg = promptInitConfig()
		}
	} else {
		fmt.Println("💬 No template found in ~/secrets/tazpod-template.yaml — configuring interactively.")
		fmt.Println("   Tip: save your defaults there to skip this step next time.")
		newCfg = promptInitConfig()
	}

	// Container name is always generated fresh per project
	newCfg.ContainerName = containerName
	newCfg.Features.GhostMode = true
	newCfg.Features.Debug = false

	data, err := yaml.Marshal(&newCfg)
	if err != nil {
		fmt.Printf("❌ Failed to serialize config: %v\n", err)
		return
	}
	if err := os.WriteFile(ConfigPath, data, 0644); err != nil {
		fmt.Printf("❌ Failed to write %s: %v\n", ConfigPath, err)
		return
	}

	fmt.Printf("✅ Project initialized.\n🐳 Container: %s\n", containerName)
}

func promptInitConfig() Config {
	fmt.Println("\n🛡️  TazPod Init — Enter your configuration")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	ask := func(prompt, defaultVal string) string {
		if defaultVal != "" {
			fmt.Printf("  %s [%s]: ", prompt, defaultVal)
		} else {
			fmt.Printf("  %s: ", prompt)
		}
		val, _ := reader.ReadString('\n')
		val = strings.TrimSpace(val)
		if val == "" {
			return defaultVal
		}
		return val
	}

	fmt.Println("── AWS SSO ──────────────────────────────")
	startURL  := ask("SSO Start URL", "")
	accountID := ask("Account ID (12 digits)", "")
	roleName  := ask("Role Name", "")
	region    := ask("Region", "eu-central-1")
	profile   := ask("SSO Profile name", "default")

	fmt.Println("\n── Docker ───────────────────────────────")
	image := ask("Image", "tazzo/tazpod-ai:latest")
	fmt.Println()

	return Config{
		Image:          image,
		User:           "tazpod",
		ActiveProvider: "home",
		Providers: map[string]ProviderConfig{
			"home": {VPNConfig: "HOME_WG_CONF"},
			"aws":  {VPNConfig: "AWS_WG_CONF"},
		},
		AwsSso: AwsSsoConfig{
			StartURL:  startURL,
			AccountID: accountID,
			RoleName:  roleName,
			Region:    region,
			Profile:   profile,
		},
	}
}

func unlock() { 
	vault.SetupIdentity()
	vault.Unlock() 
}

func askYN(question string) bool {
	fmt.Printf("%s [y/N]: ", question)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	return strings.ToLower(strings.TrimSpace(answer)) == "y"
}

func ensureContainerUp() {
	cmd := exec.Command("docker", "exec", cfg.ContainerName, "true")
	if err := cmd.Run(); err != nil {
		fmt.Println("⚠️  Container not responding. Restarting...")
		down()
		up()
		time.Sleep(2 * time.Second)
	}
}

func smartEntry() {
	loadConfigs()

	// Step 1: controlla se il progetto è inizializzato
	if _, err := os.Stat(".tazpod"); os.IsNotExist(err) {
		if !askYN("📂 No project found here. Initialize?") {
			return
		}
		initProject()
		loadConfigs()
	}

	if cfg.ContainerName == "" {
		fmt.Println("❌ container_name missing in config.yaml. Run 'tazpod init'.")
		return
	}

	// Step 2: assicura che il container sia su
	ensureContainerUp()

	// Step 3: gestione vault — tutto dentro il container (aws CLI, sudo mount disponibili lì)
	cwd, _ := os.Getwd()
	localVault := filepath.Join(cwd, ".tazpod", "vault", "vault.tar.aes")
	containerUnlocked := exec.Command("docker", "exec", cfg.ContainerName, "mountpoint", "-q", vault.MountPath).Run() == nil
	if containerUnlocked {
		// Vault già in RAM nel container, entra direttamente
	} else if utils.FileExist(localVault) {
		if askYN("🔐 Vault trovato. Unlock?") {
			execInContainer("tazpod unlock")
		}
	} else {
		if askYN("🔑 No local vault found. Bootstrap? (login + pull + unlock)") {
			// Ogni step in un exec separato: TTY pulito tra uno e l'altro
			// evita che i keystroke del SSO browser finiscano nel buffer della passphrase
			if !execInContainer("tazpod login") { goto enterContainer }
			if !execInContainer("tazpod pull vault") { goto enterContainer }
			execInContainer("tazpod unlock")
		}
	}

enterContainer:

	// Step 4: entra nel container
	enter()
}

// execInContainer esegue un comando interattivo nel container (stdin/stdout/stderr passthrough)
// execInContainer esegue un comando interattivo nel container con TTY dedicato.
// Ritorna true se il comando è uscito con successo (exit code 0).
func execInContainer(command string) bool {
	// AWS_CONFIG_FILE esplicito: bypass del symlink ~/.aws che richiede .bashrc interattivo
	cmd := exec.Command("docker", "exec", "-it",
		"-e", "AWS_CONFIG_FILE=/workspace/.tazpod/.aws/config",
		"-w", "/workspace",
		cfg.ContainerName, "bash", "-c", command)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run() == nil
}

func login() {
	profile := cfg.AwsSso.Profile
	if profile == "" { profile = "tazlab-bootstrap" }
	fmt.Printf("🔑 Authenticating with AWS SSO (profile: %s)...\n", profile)
	cmd := exec.Command("aws", "sso", "login", "--profile", profile)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ AWS SSO login failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ AWS SSO session active.")
}

func push() {
	subarg := ""
	if len(os.Args) > 2 { subarg = os.Args[2] }

	switch subarg {
	case "vault", "":
		pushVault()
	case "identity":
		pushIdentity()
	default:
		fmt.Printf("❌ Unknown push target: %s\n", subarg)
	}
}

func pushIdentity() {
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

	s3, err := utils.NewS3Client("", cfg.AwsSso.Profile)
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

	switch subarg {
	case "vault", "":
		pullVault()
	case "identity":
		pullIdentity()
	default:
		fmt.Printf("❌ Unknown pull target: %s\n", subarg)
	}
}

func pullIdentity() {
	s3, err := utils.NewS3Client("", cfg.AwsSso.Profile)
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

// loadVaultAWSCredentials carica le credenziali AWS dal vault nell'env.
// If the vault is unlocked, raw credential files are in MountPath — used instead of the SSO profile.
func loadVaultAWSCredentials() {
	if !utils.IsMounted(vault.MountPath) { return }
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join(vault.MountPath, name))
		if err != nil { return "" }
		v := strings.TrimSpace(string(data))
		// Rimuove apici singoli o doppi (es. 'value' o "value")
		for len(v) >= 2 && ((v[0] == '\'' && v[len(v)-1] == '\'') || (v[0] == '"' && v[len(v)-1] == '"')) {
			v = v[1 : len(v)-1]
		}
		return v
	}
	if ak := read("aws-access-key-id"); ak != "" { os.Setenv("AWS_ACCESS_KEY_ID", ak) }
	if sk := read("aws-secret-access-key"); sk != "" { os.Setenv("AWS_SECRET_ACCESS_KEY", sk) }
}

func pushVault() {
	loadVaultAWSCredentials()
	cwd, _ := os.Getwd()
	vaultFile := filepath.Join(cwd, ".tazpod", "vault", "vault.tar.aes")
	if !utils.FileExist(vaultFile) {
		fmt.Println("❌ No vault file found. Run 'tazpod save' first.")
		return
	}

	s3, err := utils.NewS3Client("", cfg.AwsSso.Profile)
	if err != nil {
		fmt.Printf("❌ S3 Client error: %v\n", err)
		return
	}

	fmt.Println("☁️  Uploading vault.tar.aes to S3...")
	start := time.Now()
	if err := s3.UploadFile("tazpod/vault/vault.tar.aes", vaultFile); err != nil {
		fmt.Printf("❌ Upload failed: %v\n", err)
		return
	}
	fmt.Printf("✅ Vault pushed successfully in %v.\n", time.Since(start))
}

func pullVault() {
	loadVaultAWSCredentials()
	cwd, _ := os.Getwd()
	vaultFile := filepath.Join(cwd, ".tazpod", "vault", "vault.tar.aes")

	s3, err := utils.NewS3Client("", cfg.AwsSso.Profile)
	if err != nil {
		fmt.Printf("❌ S3 Client error: %v\n", err)
		os.Exit(1)
	}

	os.MkdirAll(filepath.Join(cwd, ".tazpod", "vault"), 0755)
	fmt.Println("☁️  Downloading vault.tar.aes from S3...")
	if err := s3.DownloadFile("tazpod/vault/vault.tar.aes", vaultFile); err != nil {
		fmt.Printf("❌ Download failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Vault pulled. Run 'tazpod unlock' to decrypt.")
}

func printExportEnv() {
	// Placeholder: AWS SSO credentials are managed by the SDK credential chain.
}
