package vault

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
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
	VaultDir         = "/workspace/.tazpod/vault"
	VaultFile        = VaultDir + "/vault.tar.aes"
	ContentHashFile  = VaultDir + "/last-content.hash"
	MountPath        = "/home/tazpod/secrets"

	// AWS Enclave paths
	AwsLocalHome = "/home/tazpod/.aws"
	AwsVaultDir  = MountPath + "/.aws"

	PassCache = MountPath + "/.vault_pass"
)

var cachedPassphrase string

func Unlock() {
	if utils.IsMounted(MountPath) {
		fmt.Println("✅ Vault already unlocked (RAM).")
		loadCachedPass()
		SetupIdentity()
		if err := setupBindAuth(); err != nil {
			fatal(err.Error())
		}
		return
	}

	fmt.Println("🔐 TAZPOD UNLOCK (RAM MODE)")

	if err := mountRAM(); err != nil {
		fatal(err.Error())
	}

	if utils.FileExist(VaultFile) {
		data, err := os.ReadFile(VaultFile)
		if err != nil {
			fatal(err.Error())
		}

		var decrypted []byte
		for attempts := 3; attempts > 0; attempts-- {
			cachedPassphrase = getPassphrase()
			decrypted, err = crypto.Decrypt(data, cachedPassphrase)
			if err == nil {
				break
			}
			slog.Error("Wrong passphrase", "attempts_remaining", attempts-1)
			if attempts == 1 {
				unmountRAM()
				fatal("Too many failed attempts. Vault locked.")
			}
		}

		fmt.Print("📂 Loading vault... ")
		if err := Untar(decrypted, MountPath); err != nil {
			fatal(err.Error())
		}
		fmt.Println("✅ OK")
	} else {
		cachedPassphrase = getPassphrase()
		fmt.Println("🆕 New vault initialized.")
	}

	if err := os.WriteFile(PassCache, []byte(cachedPassphrase), 0600); err != nil {
		slog.Warn("Could not cache passphrase", "error", err)
	}
	SetupIdentity()
	if err := setupBindAuth(); err != nil {
		fatal(err.Error())
	}
}

func SetupIdentity() {
	// Ensure AI tool config dirs exist in workspace (mirrored by .bashrc symlinks)
	for _, dir := range []string{".pi", ".omp", ".gemini", ".claude", ".aws", ".opencode"} {
		os.MkdirAll("/workspace/.tazpod/"+dir, 0755)
	}

	exec.Command("sudo", "chown", "-R", "tazpod:tazpod", "/workspace/.tazpod").Run()
}

func Save(passphrase string) (string, error) {
	if !utils.IsMounted(MountPath) {
		fmt.Println("⚠️  Vault is not mounted.")
		return "", fmt.Errorf("vault not mounted")
	}

	loadCachedPass()
	if passphrase == "" {
		passphrase = cachedPassphrase
	}

	if passphrase == "" {
		fmt.Print("💾 Enter passphrase to SAVE: ")
		b, _ := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		passphrase = string(b)
		cachedPassphrase = passphrase
		if err := os.WriteFile(PassCache, []byte(passphrase), 0600); err != nil {
			slog.Warn("Could not cache passphrase", "error", err)
		}
	}

	fmt.Print("💾 Saving vault to disk... ")
	rawBytes, err := TarDir(MountPath)
	if err != nil {
		slog.Error("Pack error", "error", err)
		return "", err
	}

	// Content hash del plaintext (prima della crittografia)
	h := sha256.Sum256(rawBytes)
	contentHash := hex.EncodeToString(h[:])

	if err := os.MkdirAll(VaultDir, 0755); err != nil {
		slog.Error("Cannot create vault dir", "error", err)
		return contentHash, err
	}

	if err := os.WriteFile(ContentHashFile, []byte(contentHash), 0644); err != nil {
		slog.Warn("Cannot write content hash", "error", err)
	}

	encrypted, err := crypto.Encrypt(rawBytes, passphrase)
	if err != nil {
		slog.Error("Encrypt error", "error", err)
		return contentHash, err
	}

	if err := os.WriteFile(VaultFile, encrypted, 0644); err != nil {
		slog.Error("Cannot write vault file", "error", err)
		return contentHash, err
	}
	fmt.Println("✅ Saved.")
	return contentHash, nil
}

func loadCachedPass() {
	if cachedPassphrase != "" {
		return
	}
	if data, err := os.ReadFile(PassCache); err == nil {
		cachedPassphrase = string(data)
	}
}

func setupBindAuth() error {
	fmt.Println("🔗 Bridging AWS Enclave...")
	if err := os.MkdirAll(AwsVaultDir, 0700); err != nil {
		return fmt.Errorf("cannot create AWS vault dir: %w", err)
	}

	if err := bridge(AwsLocalHome, AwsVaultDir); err != nil {
		return err
	}
	return nil
}

func bridge(local, vault string) error {
	if utils.IsMounted(local) {
		exec.Command("sudo", "umount", "-l", local).Run()
	}
	exec.Command("sudo", "rm", "-rf", local).Run()
	if err := os.MkdirAll(local, 0755); err != nil {
		return fmt.Errorf("cannot create bind target %s: %w", local, err)
	}

	fmt.Printf("  -> Binding %s\n", local)
	if err := exec.Command("sudo", "mount", "--bind", vault, local).Run(); err != nil {
		return fmt.Errorf("bind mount failed for %s -> %s: %w", vault, local, err)
	}
	if !utils.IsMounted(local) {
		return fmt.Errorf("bind mount did not materialize at %s", local)
	}
	return nil
}

func Lock() {
	if !utils.IsMounted(MountPath) {
		return
	}
	fmt.Println("🔒 Locking vault...")
	exec.Command("sudo", "umount", "-l", AwsLocalHome).Run()
	unmountRAM()
}

func mountRAM() error {
	if err := os.MkdirAll(MountPath, 0755); err != nil {
		return fmt.Errorf("cannot create vault mount path: %w", err)
	}
	exec.Command("sudo", "umount", "-l", MountPath).Run()
	cmd := exec.Command("sudo", "mount", "-t", "tmpfs", "-o", "size=64M,mode=0700,uid=1000,gid=1000", "tmpfs", MountPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tmpfs mount failed at %s: %w", MountPath, err)
	}
	if !utils.IsMounted(MountPath) {
		return fmt.Errorf("tmpfs mount did not materialize at %s", MountPath)
	}
	return nil
}

func unmountRAM() {
	exec.Command("sudo", "umount", "-l", MountPath).Run()
}

func getPassphrase() string {
	if _, err := os.Stat(VaultFile); err == nil {
		fmt.Print("🔑 Enter Passphrase: ")
		p, _ := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		return string(p)
	}
	for {
		fmt.Print("📝 Define NEW Passphrase: ")
		p1, _ := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		fmt.Print("📝 Confirm Passphrase: ")
		p2, _ := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if string(p1) == string(p2) && len(p1) > 0 {
			return string(p1)
		}
		fmt.Println("❌ Mismatch. Try again.")
	}
}

func fatal(msg string) {
	slog.Error(msg)
	os.Exit(1)
}

func Untar(data []byte, dest string) error {
	gr, err := gzip.NewReader(io.NopCloser(strings.NewReader(string(data))))
	if err != nil {
		return err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dest, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
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
		if err != nil || path == src {
			return err
		}

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
	tw.Close()
	gw.Close()
	return buf.Bytes(), nil
}
