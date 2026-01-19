package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
	"golang.org/x/term"
)

// --- CONFIGURATION STRUCT ---
type Config struct {
	Image         string `yaml:"image"`
	ContainerName string `yaml:"container_name"`
	User          string `yaml:"user"`
	Build         struct {
		Dockerfile string `yaml:"dockerfile"`
		Context    string `yaml:"context"`
	} `yaml:"build"`
	Features struct {
		GhostMode bool `yaml:"ghost_mode"`
	} `yaml:"features"`
}

var cfg = Config{
	Image:         "tazzo/tazlab.net:tazpod-base",
	ContainerName: "tazpod-lab",
	User:          "tazpod",
}

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
)

func main() {
	if len(os.Args) < 2 {
		help()
		os.Exit(1)
	}

	loadConfig()

	switch os.Args[1] {
	case "up":
		up()
	case "down":
		down()
	case "enter", "ssh":
		enter()
	case "init":
		initProject()
	case "unlock":
		unlock()
	case "lock":
		lock()
	case "reinit":
		reinit()
	case "login":
		login()
	case "internal-ghost":
		internalGhost()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func loadConfig() {
	data, err := os.ReadFile(ConfigPath)
	if err == nil {
		yaml.Unmarshal(data, &cfg)
	}
}

func help() {
	fmt.Println("Usage: tazpod <command>")
	fmt.Println("  init    -> Create .tazpod config")
	fmt.Println("  up      -> Start container")
	fmt.Println("  down    -> Stop container")
	fmt.Println("  ssh     -> Enter container")
	fmt.Println("  unlock  -> Unlock Vault")
	fmt.Println("  login   -> Infisical Login & Init (Inside Ghost Mode)")
}

// --- HOST COMMANDS ---

func initProject() {
	if _, err := os.Stat(".tazpod"); err == nil {
		fmt.Println("⚠️  .tazpod already exists.")
		return
	}
	os.Mkdir(".tazpod", 0755)
	
	yamlContent := `# TazPod Configuration
image: "tazzo/tazlab.net:tazpod-base"
container_name: "tazpod-lab"
user: "tazpod"
features:
  ghost_mode: true
`
	os.WriteFile(".tazpod/config.yaml", []byte(yamlContent), 0644)
	fmt.Println("✅ Initialized .tazpod/config.yaml")
}

func up() {
	fmt.Printf("🏗️  TazPod Up [%s]...\n", cfg.ContainerName)

	if cfg.Build.Dockerfile != "" {
		fmt.Printf("🔨 Building from %s...\n", cfg.Build.Dockerfile)
		ctx := "."
		if cfg.Build.Context != "" { ctx = cfg.Build.Context }
		runCmd("docker", "build", "-f", cfg.Build.Dockerfile, "-t", cfg.Image, ctx)
	}

	fmt.Println("🛑 Cleaning old instances...")
	exec.Command("docker", "rm", "-f", cfg.ContainerName).Run()
	
	cwd, _ := os.Getwd()
	fmt.Printf("🚀 Starting %s...\n", cfg.Image)
	
	display := os.Getenv("DISPLAY")
	xauth := os.Getenv("XAUTHORITY")
	if xauth == "" { xauth = os.Getenv("HOME") + "/.Xauthority" }

	runCmd("docker", "run", "-d", 
		"--name", cfg.ContainerName, 
		"--privileged", 
		"--network", "host", 
		"-e", "DISPLAY="+display,
		"-e", "XAUTHORITY=/home/tazpod/.Xauthority",
		"-v", "/tmp/.X11-unix:/tmp/.X11-unix",
		"-v", xauth+":/home/tazpod/.Xauthority",
		"-v", cwd+":/workspace", 
		"-w", "/workspace", 
		cfg.Image, "sleep", "infinity")
	
	fmt.Println("✅ Ready.")
}

func down() {
	fmt.Println("🧹 Shutting down...")
	runCmd("docker", "rm", "-f", cfg.ContainerName)
}

func enter() {
	binary, _ := exec.LookPath("docker")
	args := []string{"docker", "exec", "-it", cfg.ContainerName, "bash"}
	syscall.Exec(binary, args, os.Environ())
}

// --- CONTAINER COMMANDS ---

func unlock() {
	if os.Getenv(GhostEnvVar) == "true" {
		fmt.Println("✅ Already in Ghost Mode.")
		return
	}
	fmt.Println("👻 Entering Ghost Mode...")
	cmd := exec.Command("sudo", "unshare", "--mount", "--propagation", "private", "/usr/local/bin/tazpod", "internal-ghost")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	
	if _, statErr := os.Stat(StayMarker); statErr == nil {
		os.Remove(StayMarker)
		os.Exit(2)
	}
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok { os.Exit(exitError.ExitCode()) }
		os.Exit(1)
	}
}

func internalGhost() {
	if os.Geteuid() != 0 { fmt.Println("❌ Root required."); os.Exit(1) }
	fmt.Println("🔐 TAZPOD UNLOCK")

	var passphrase string
	if !fileExist(VaultPath) {
		fmt.Println("🆕 New vault setup.")
		for {
			fmt.Print("📝 Define Passphrase: ")
			p1, _ := term.ReadPassword(int(syscall.Stdin)); fmt.Println()
			fmt.Print("📝 Confirm: ")
			p2, _ := term.ReadPassword(int(syscall.Stdin)); fmt.Println()
			if string(p1) == string(p2) && len(p1) > 0 { passphrase = string(p1); break }
		}
	} else {
		fmt.Print("🔑 Passphrase: ")
		p, _ := term.ReadPassword(int(syscall.Stdin)); fmt.Println()
		passphrase = string(p)
	}

	exec.Command("mknod", "/dev/loop-control", "c", "10", "237").Run()
	for i := 0; i < 64; i++ { exec.Command("mknod", fmt.Sprintf("/dev/loop%d", i), "b", "7", fmt.Sprintf("%d", i)).Run() }
	os.MkdirAll(VaultDir, 0755)
	cleanupMappers()
	exec.Command("bash", "-c", "losetup -a | grep 'vault.img' | cut -d: -f1 | xargs -r losetup -d").Run()

	mapperPath := "/dev/mapper/" + MapperName
	loopDev := runOutput("losetup", "-f", "--show", VaultPath) 

	if !fileExist(VaultPath) {
		runCmd("dd", "if=/dev/zero", "of="+VaultPath, "bs=1M", "count="+VaultSizeMB, "status=none")
		loopDev = runOutput("losetup", "-f", "--show", VaultPath)
		runWithStdin(passphrase, "cryptsetup", "luksFormat", "--batch-mode", loopDev)
		runWithStdin(passphrase, "cryptsetup", "open", loopDev, MapperName)
		exec.Command("dmsetup", "mknodes").Run()
		waitForDevice(mapperPath)
		runCmd("mkfs.ext4", "-q", mapperPath)
	} else {
		if _, err := runWithStdin(passphrase, "cryptsetup", "open", loopDev, MapperName); err != nil {
			fmt.Println("❌ Decrypt Failed."); runCmd("losetup", "-d", loopDev); os.Exit(1)
		}
		exec.Command("dmsetup", "mknodes").Run()
		waitForDevice(mapperPath)
	}

	os.MkdirAll(MountPath, 0755)
	runCmd("mount", "-t", "ext4", mapperPath, MountPath)
	runCmd("chown", fmt.Sprintf("%d:%d", TazPodUID, TazPodGID), MountPath)

	// RESTORE INFISICAL SESSION
	restoreAuth()

	fmt.Println("\n✅ GHOST MODE ACTIVE.")
	bashCmd := exec.Command("bash")
	bashCmd.Stdin, bashCmd.Stdout, bashCmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	bashCmd.SysProcAttr = &syscall.SysProcAttr{ Credential: &syscall.Credential{Uid: uint32(TazPodUID), Gid: uint32(TazPodGID)} }
	bashCmd.Env = append(os.Environ(), GhostEnvVar+"=true", "USER=tazpod", "HOME=/home/tazpod")
	bashCmd.Run()

	// PERSIST INFISICAL SESSION ON EXIT
persistAuth()

	fmt.Println("\n🔒 Cleanup...")
	exec.Command("umount", "-f", MountPath).Run()
	cleanupMappers()
	exec.Command("bash", "-c", "losetup -a | grep 'vault.img' | cut -d: -f1 | xargs -r losetup -d").Run()
	fmt.Println("✅ Vault locked.")
}

func login() {
	if os.Getenv(GhostEnvVar) != "true" {
		fmt.Println("❌ Please run 'tazpod unlock' first to enter Ghost Mode.")
		os.Exit(1)
	}
	
	fmt.Println("🔐 Infisical Login Sequence...")
	if err := runCmdInteractive("infisical", "login"); err != nil {
		fmt.Println("❌ Login failed.")
		return
	}
	
	fmt.Println("🛠️  Infisical Init...")
	if err := runCmdInteractive("infisical", "init"); err != nil {
		fmt.Println("❌ Init failed.")
		return
	}
	
	// Force persistence check immediately
	// Note: Actual persistence happens when internalGhost exits (calls persistAuth), 
	// but we can also do it here if we want immediate sync to disk.
	fmt.Println("✅ Login successful. Session will be saved to vault on exit.")
}

// Moves ~/.infisical -> ~/secrets/.auth-infisical
func persistAuth() {
	if _, err := os.Stat(InfisicalDir); err == nil {
		// Se esiste la cartella locale (nuovo login), copiala nel vault
		// Attenzione: se è un symlink, non fare nulla (è già nel vault)
		info, _ := os.Lstat(InfisicalDir)
		if info.Mode()&os.ModeSymlink == 0 {
			// È una directory reale. Spostiamola.
			os.RemoveAll(VaultAuthDir)
			exec.Command("cp", "-r", InfisicalDir, VaultAuthDir).Run()
			os.RemoveAll(InfisicalDir)
			os.Symlink(VaultAuthDir, InfisicalDir)
			fmt.Println("💾 Infisical session secured in vault.")
		}
	}
}

// Links ~/secrets/.auth-infisical -> ~/.infisical
func restoreAuth() {
	if _, err := os.Stat(VaultAuthDir); err == nil {
		os.RemoveAll(InfisicalDir)
		os.Symlink(VaultAuthDir, InfisicalDir)
		// Fix permissions just in case
		os.Chown(InfisicalDir, TazPodUID, TazPodGID)
	}
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
	if strings.ToLower(c) == "y" { os.Remove(VaultPath); unlock() }
}

func runCmd(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.Run()
}

func runCmdInteractive(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func runOutput(name string, args ...string) string {
	out, _ := exec.Command(name, args...).Output()
	return strings.TrimSpace(string(out))
}

func runWithStdin(input, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = bytes.NewBufferString(input)
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	err := cmd.Run()
	return out.String(), err
}

func fileExist(path string) bool { _, err := os.Stat(path); return err == nil }
func waitForDevice(path string) { for i:=0; i<20; i++ { if fileExist(path) { return }; time.Sleep(200*time.Millisecond) } }
