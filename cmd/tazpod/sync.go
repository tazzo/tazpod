package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"tazpod/internal/utils"
	"tazpod/internal/vault"
)

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

func pull() {
	subarg := ""
	if len(os.Args) > 2 {
		subarg = os.Args[2]
	}

	switch subarg {
	case "vault", "":
		pullVault()
	default:
		fmt.Printf("❌ Unknown pull target: %s\n", subarg)
	}
}

func push() {
	subarg := ""
	if len(os.Args) > 2 {
		subarg = os.Args[2]
	}

	switch subarg {
	case "vault", "":
		pushVault()
	default:
		fmt.Printf("❌ Unknown push target: %s\n", subarg)
	}
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

func pushVault() {
	loadVaultAWSCredentials()
	start := time.Now()
	cwd, _ := os.Getwd()
	vaultFile := filepath.Join(cwd, ".tazpod", "vault", "vault.tar.aes")

	if _, err := os.Stat(vaultFile); os.IsNotExist(err) {
		fmt.Println("❌ No vault file found in .tazpod/vault/vault.tar.aes")
		return
	}

	s3, err := utils.NewS3Client("", cfg.AwsSso.Profile)
	if err != nil {
		fmt.Printf("❌ S3 Client error: %v\n", err)
		return
	}

	fmt.Println("☁️  Uploading vault.tar.aes to S3...")
	if err := s3.UploadFile("tazpod/vault/vault.tar.aes", vaultFile); err != nil {
		fmt.Printf("❌ Upload failed: %v\n", err)
		return
	}
	fmt.Printf("✅ Vault pushed successfully in %v.\n", time.Since(start))
}
