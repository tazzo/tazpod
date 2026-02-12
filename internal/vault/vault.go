package vault

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"tazpod/internal/crypto"
	"tazpod/internal/utils"

	"golang.org/x/term"
)

const (
	VaultDir      = "/workspace/.tazpod/vault"
	VaultFile     = VaultDir + "/vault.tar.aes"
	MountPath     = "/home/tazpod/secrets"
	SecretsYAML   = "/workspace/.tazpod/secrets-sync-config.yml"
	
	// Percorsi speculari per Infisical
	InfisicalLocalHome    = "/home/tazpod/.infisical"
	InfisicalKeyringLocal = "/home/tazpod/infisical-keyring"
	GeminiLocalHome       = "/home/tazpod/.gemini"
	
	// Cartelle nel Vault (RAM) - Nomi puliti
	InfisicalVaultDir     = MountPath + "/.infisical"
	InfisicalKeyringVault = MountPath + "/infisical-keyring"
	GeminiVaultDir        = "/workspace/.tazpod/.gemini"
	
	PassCache             = MountPath + "/.vault_pass"
)

var cachedPassphrase string

func Unlock() {
	if utils.IsMounted(MountPath) {
		fmt.Println("✅ Vault already unlocked (RAM).")
		loadCachedPass()
		SetupIdentity()
		setupBindAuth()
		return
	}

	fmt.Println("🔐 TAZPOD UNLOCK (RAM MODE)")
	cachedPassphrase = getPassphrase()

	mountRAM()

	if utils.FileExist(VaultFile) {
		fmt.Print("📂 Loading vault... ")
		data, err := os.ReadFile(VaultFile)
		if err != nil { fatal(err.Error()) }
		
		decrypted, err := crypto.Decrypt(data, cachedPassphrase)
		if err != nil {
			fmt.Println("❌ WRONG PASSWORD.")
			unmountRAM()
			os.Exit(1)
		}
		
		if err := Untar(decrypted, MountPath); err != nil { fatal(err.Error()) }
		fmt.Println("✅ OK")
	} else {
		fmt.Println("🆕 New vault initialized.")
	}

	os.WriteFile(PassCache, []byte(cachedPassphrase), 0600)
	SetupIdentity()
	setupBindAuth()
}

func SetupIdentity() {
	// Gemini Link (History)
	os.MkdirAll(GeminiVaultDir, 0755)
	exec.Command("sudo", "chown", "-R", "tazpod:tazpod", "/workspace/.tazpod").Run()
	if _, err := os.Lstat(GeminiLocalHome); err == nil {
		exec.Command("sudo", "rm", "-rf", GeminiLocalHome).Run()
	}
	os.Symlink(GeminiVaultDir, GeminiLocalHome)
}

func Save(passphrase string) {
	if !utils.IsMounted(MountPath) {
		fmt.Println("⚠️  Vault is not mounted.")
		return
	}

	loadCachedPass()
	if passphrase == "" { passphrase = cachedPassphrase }

	if passphrase == "" {
		fmt.Print("💾 Enter passphrase to SAVE: ")
		b, _ := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		passphrase = string(b)
		cachedPassphrase = passphrase
		os.WriteFile(PassCache, []byte(passphrase), 0600)
	}

	fmt.Print("💾 Saving vault to disk... ")
	rawBytes, err := TarDir(MountPath)
	if err != nil { fmt.Println("❌ Pack error:", err); return }

	encrypted, err := crypto.Encrypt(rawBytes, passphrase)
	if err != nil { fmt.Println("❌ Encrypt error:", err); return }

	os.MkdirAll(VaultDir, 0755)
	os.WriteFile(VaultFile, encrypted, 0644)
	fmt.Println("✅ Saved.")
}

func loadCachedPass() {
	if cachedPassphrase != "" { return }
	if data, err := os.ReadFile(PassCache); err == nil {
		cachedPassphrase = string(data)
	}
}

func setupBindAuth() {
	fmt.Println("🔗 Bridging Enclave Auth...")
	os.MkdirAll(InfisicalVaultDir, 0700)
	os.MkdirAll(InfisicalKeyringVault, 0700)

	bridge(InfisicalLocalHome, InfisicalVaultDir)
	bridge(InfisicalKeyringLocal, InfisicalKeyringVault)
}

func bridge(local, vault string) {
	if utils.IsMounted(local) {
		exec.Command("sudo", "umount", "-l", local).Run()
	}
	exec.Command("sudo", "rm", "-rf", local).Run()
	os.MkdirAll(local, 0755)
	
	fmt.Printf("  -> Binding %s\n", local)
	exec.Command("sudo", "mount", "--bind", vault, local).Run()
}

func Lock() {
	if !utils.IsMounted(MountPath) { return }
	fmt.Println("🔒 Locking vault...")
	exec.Command("sudo", "umount", "-l", InfisicalLocalHome).Run()
	exec.Command("sudo", "umount", "-l", InfisicalKeyringLocal).Run()
	unmountRAM()
}

func mountRAM() {
	os.MkdirAll(MountPath, 0755)
	exec.Command("sudo", "umount", "-l", MountPath).Run()
	cmd := exec.Command("sudo", "mount", "-t", "tmpfs", "-o", "size=64M,mode=0700,uid=1000,gid=1000", "tmpfs", MountPath)
	cmd.Run()
}

func unmountRAM() {
	exec.Command("sudo", "umount", "-l", MountPath).Run()
}

func getPassphrase() string {
	if _, err := os.Stat(VaultFile); err == nil {
		fmt.Print("🔑 Enter Passphrase: ")
		p, _ := term.ReadPassword(int(syscall.Stdin))
		fmt.Println(); return string(p)
	}
	for {
		fmt.Print("📝 Define NEW Passphrase: ")
		p1, _ := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		fmt.Print("📝 Confirm Passphrase: ")
		p2, _ := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if string(p1) == string(p2) && len(p1) > 0 { return string(p1) }
		fmt.Println("❌ Mismatch. Try again.")
	}
}

func fatal(msg string) { fmt.Println("❌ " + msg); os.Exit(1) }

func Untar(data []byte, dest string) error {
	gr, err := gzip.NewReader(io.NopCloser(strings.NewReader(string(data))))
	if err != nil { return err }
	defer gr.Close()
	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF { break }
		if err != nil { return err }
		target := filepath.Join(dest, header.Name)
		switch header.Typeflag {
		case tar.TypeDir: os.MkdirAll(target, 0755)
		case tar.TypeReg:
			f, _ := os.Create(target)
			io.Copy(f, tr)
			f.Close()
			os.Chown(target, 1000, 1000)
			os.Chmod(target, os.FileMode(header.Mode))
		}
	}
	return nil
}

func TarDir(src string) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil || path == src { return err }
		relPath, _ := filepath.Rel(src, path)
		header, _ := tar.FileInfoHeader(info, relPath)
		header.Name = relPath
		tw.WriteHeader(header)
		if !info.IsDir() {
			data, _ := os.Open(path)
			io.Copy(tw, data)
			data.Close()
		}
		return nil
	})
	tw.Close(); gw.Close()
	return buf.Bytes(), nil
}

// PackageIdentity bundles the entire .tazpod directory for S3 sync
func PackageIdentity() ([]byte, error) {
	fmt.Println("📦 Packaging identity (.tazpod)...")
	return TarDir(".tazpod")
}

// ExtractIdentity extracts the identity bundle from S3
func ExtractIdentity(data []byte) error {
	fmt.Println("📂 Extracting identity (.tazpod)...")
	return Untar(data, ".")
}

