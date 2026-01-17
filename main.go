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

	"golang.org/x/term"
)

const (
	ContainerName = "tazpod-lab"
	ImageName     = "tazpod-engine:local"
	VaultDir      = "/workspace/.tazpod-vault"
	VaultPath     = VaultDir + "/vault.img"
	MountPath     = "/home/tazpod/secrets"
	MapperName    = "tazpod_vault"
	VaultSizeMB   = "512"
	SecretHashKey = "TAZPOD_HASH_V4"
	SecretsYAML   = "/workspace/secrets.yml"
)

var ProjectID = ""

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: tazpod [up|down|unlock|lock|sync|env]")
		os.Exit(1)
	}

	command := os.Args[1]
	if command == "unlock" || command == "sync" || command == "env" {
		loadConfig()
	}

	switch command {
	case "up":
		up()
	case "down":
		down()
	case "unlock":
		unlock()
	case "lock":
		lock()
	case "sync":
		sync()
	case "env":
		loadEnv()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}

func loadConfig() {
	if _, err := os.Stat(SecretsYAML); err == nil {
		out, _ := exec.Command("yq", ".config.infisical_project_id", SecretsYAML).Output()
		ProjectID = clean(string(out))
	}
}

func clean(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.ReplaceAll(s, "'", "")
	if s == "null" { return "" }
	return s
}

// --- HOST COMMANDS ---

func up() {
	fmt.Println("🏗️  Ensuring TazPod Image...")
	runCmd("docker", "build", "-f", "Dockerfile.base", "-t", ImageName, ".")

	fmt.Println("🛑 Cleaning old instances...")
	exec.Command("docker", "rm", "-f", ContainerName).Run()

	cwd, _ := os.Getwd()
	fmt.Printf("🚀 Launching TazPod in %s...\n", cwd)
	
	runCmd("docker", "run", "-d",
		"--name", ContainerName,
		"--privileged",
		"--network", "host",
		"-v", cwd+":/workspace",
		"-w", "/workspace",
		ImageName, "sleep", "infinity")

	fmt.Println("✅ TazPod is running.")
	fmt.Printf("👉 Entry: docker exec -it %s bash\n", ContainerName)
}

func down() {
	fmt.Println("🧹 Shutting down TazPod...")
	runCmd("docker", "rm", "-f", ContainerName)
	fmt.Println("✅ Done.")
}

// --- CONTAINER COMMANDS ---

func unlock() {
	if isMounted(MountPath) {
		fmt.Println("✅ Vault already unlocked.")
		restoreAuthPersistence() // Assicura link se già montato
		return
	}

	fmt.Println("🔐 TAZPOD UNLOCK")

	hashVal := getHash()
	if hashVal == "" && !fileExist(VaultPath) {
		fmt.Println("🔄 First setup detected. Infisical Login required...")
		runCmdInteractive("infisical", "login")
		hashVal = getHash()
	}

	var passphrase string
	for {
		fmt.Print("🔑 Enter Master Passphrase: ")
		bytePass, _ := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		passphrase = string(bytePass)

		if hashVal != "" && strings.Contains(hashVal, "$") {
			parts := strings.Split(hashVal, "$")
			if len(parts) > 2 {
				salt := parts[2]
				checkOut, err := runWithStdin(passphrase, "openssl", "passwd", "-6", "-salt", salt, "-stdin")
				if err == nil && strings.TrimSpace(checkOut) == hashVal {
					break
				}
				fmt.Println("❌ WRONG PASSPHRASE. Try again.")
			} else {
				break
			}
		} else {
			if !fileExist(VaultPath) {
				fmt.Print("📝 Confirm New Passphrase: ")
				bytePassConf, _ := term.ReadPassword(int(syscall.Stdin))
				fmt.Println()
				if passphrase == string(bytePassConf) {
					newHash, _ := runWithStdin(passphrase, "openssl", "passwd", "-6", "-stdin")
					hashVal = strings.TrimSpace(newHash)
					if ProjectID != "" {
						fmt.Println("📦 Securing hash in Infisical...")
					exec.Command("infisical", "secrets", "set", SecretHashKey+"="+hashVal, "--projectId", ProjectID).Run()
					}
					break
				}
				fmt.Println("❌ Passwords do not match.")
			} else {
				break
			}
		}
	}

	// Device Nodes Prep
	exec.Command("sudo", "mkdir", "-p", "/dev/mapper").Run()
	exec.Command("sudo", "mknod", "/dev/mapper/control", "c", "10", "236").Run()
	exec.Command("sudo", "mknod", "/dev/loop-control", "c", "10", "237").Run()
	for i := 0; i < 64; i++ {
		exec.Command("sudo", "mknod", fmt.Sprintf("/dev/loop%%d", i), "b", "7", fmt.Sprintf("%d", i)).Run()
	}

	os.MkdirAll(VaultDir, 0755)
	exec.Command("bash", "-c", "sudo losetup -a | grep 'vault.img' | cut -d: -f1 | xargs -r sudo losetup -d").Run()

	mapperPath := "/dev/mapper/" + MapperName

	if !fileExist(VaultPath) {
		fmt.Println("🆕 Creating encrypted vault file...")
		runCmd("dd", "if=/dev/zero", "of="+VaultPath, "bs=1M", "count="+VaultSizeMB, "status=none")
		loopDev := runOutput("sudo", "losetup", "-f", "--show", VaultPath)
		runWithStdin(passphrase, "sudo", "cryptsetup", "luksFormat", "--batch-mode", loopDev)
		runWithStdin(passphrase, "sudo", "cryptsetup", "open", loopDev, MapperName)
		exec.Command("sudo", "dmsetup", "mknodes").Run()
		waitForDevice(mapperPath)
		runCmd("sudo", "mkfs.ext4", "-q", mapperPath)
	} else {
		fmt.Println("💾 Unlocking existing vault...")
		loopDev := runOutput("sudo", "losetup", "-f", "--show", VaultPath)
		if _, err := runWithStdin(passphrase, "sudo", "cryptsetup", "open", loopDev, MapperName); err != nil {
			fmt.Println("❌ DECRYPTION FAILED.")
			runCmd("sudo", "losetup", "-d", loopDev)
			os.Exit(1)
		}
		exec.Command("sudo", "dmsetup", "mknodes").Run()
		waitForDevice(mapperPath)
	}

	// Mount
	os.MkdirAll(MountPath, 0755)
	runCmd("sudo", "mount", "-t", "ext4", mapperPath, MountPath)
	runCmd("sudo", "chown", "tazpod:tazpod", MountPath)

	// PERSISTENCE LOGIC: Salva la sessione di Infisical nel vault
	restoreAuthPersistence()

	sync()
	fmt.Println("\n✅ TAZPOD SECURED.")
}

func restoreAuthPersistence() {
	homeInfisical := "/home/tazpod/.infisical"
	vaultInfisical := filepath.Join(MountPath, ".auth-infisical")

	// Se esiste la cartella fisica in home (appena loggato), spostala nel vault
	if stat, err := os.Lstat(homeInfisical); err == nil && !stat.Mode().IsDir() == false {
		if _, isLink := os.Lstat(homeInfisical); isLink != nil {
			// È una directory reale, spostiamola
			exec.Command("rm", "-rf", vaultInfisical).Run()
			exec.Command("cp", "-r", homeInfisical, vaultInfisical).Run()
			exec.Command("rm", "-rf", homeInfisical).Run()
		}
	}
	
	// Crea il link simbolico se il vault ha i dati
	if fileExist(vaultInfisical) {
		os.RemoveAll(homeInfisical)
		os.Symlink(vaultInfisical, homeInfisical)
	}
}

func getHash() string {
	if ProjectID == "" || ProjectID == "null" { return "" }
	out, err := exec.Command("infisical", "secrets", "get", SecretHashKey, "--projectId", ProjectID, "--plain").Output()
	if err != nil { return "" }
	return strings.TrimSpace(string(out))
}

func sync() {
	if !isMounted(MountPath) || ProjectID == "" { return }
	fmt.Println("📦 Syncing secrets...")
	
	envFile := filepath.Join(MountPath, ".env-infisical")
	out, _ := exec.Command("infisical", "export", "--projectId", ProjectID, "--format=dotenv", "--silent").Output()
	os.WriteFile(envFile, out, 0600)

	if fileExist(SecretsYAML) {
		countStr, _ := exec.Command("yq", ".secrets | length", SecretsYAML).Output()
		var count int
		fmt.Sscanf(string(countStr), "%d", &count)

		f, _ := os.OpenFile(envFile, os.O_APPEND|os.O_WRONLY, 0600)
		defer f.Close()

		for i := 0; i < count; i++ {
			sName, _ := exec.Command("yq", fmt.Sprintf(".secrets[%%d].name", i), SecretsYAML).Output()
			sFile, _ := exec.Command("yq", fmt.Sprintf(".secrets[%%d].file", i), SecretsYAML).Output()
			sEnv, _ := exec.Command("yq", fmt.Sprintf(".secrets[%%d].env", i), SecretsYAML).Output()
			name, file, env := clean(string(sName)), clean(string(sFile)), clean(string(sEnv))

			val, err := exec.Command("infisical", "secrets", "get", name, "--projectId", ProjectID, "--plain").Output()
			if err == nil {
				target := filepath.Join(MountPath, file)
				os.WriteFile(target, val, 0600)
				if env != "" { f.WriteString(fmt.Sprintf("export %%s=\"%%s\"\n", env, target)) }
			}
		}
	}
}

func lock() {
	if !isMounted(MountPath) { return }
	fmt.Println("🔒 Locking TazPod...")
	exec.Command("sudo", "umount", "-f", MountPath).Run()
	exec.Command("sudo", "cryptsetup", "close", MapperName).Run()
	exec.Command("bash", "-c", "sudo losetup -a | grep 'vault.img' | cut -d: -f1 | xargs -r sudo losetup -d").Run()
	fmt.Println("✅ Vault locked.")
}

func runCmd(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func runCmdInteractive(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func runOutput(name string, args ...string) string {
	out, err := exec.Command(name, args...).Output()
	if err != nil { return "" }
	return strings.TrimSpace(string(out))
}

func runWithStdin(input, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = bytes.NewBufferString(input)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil { return stderr.String(), err }
	return out.String(), nil
}

func isMounted(path string) bool {
	out, _ := exec.Command("mount").Output()
	return strings.Contains(string(out), path)
}

func fileExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func waitForDevice(path string) {
	for i := 0; i < 20; i++ {
		if _, err := os.Stat(path); err == nil { return }
		time.Sleep(200 * time.Millisecond)
	}
}

func loadEnv() {
	envFile := filepath.Join(MountPath, ".env-infisical")
	if fileExist(envFile) {
		out, _ := os.ReadFile(envFile)
		fmt.Print(string(out))
	}
}