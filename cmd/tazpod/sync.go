package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"tazpod/internal/utils"
	"tazpod/internal/vault"
)

func syncDaemon() {
	syncLog := "/tmp/tazpod-sync.log"
	f, err := os.OpenFile(syncLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("❌ Failed to open sync log: %v\n", err)
		return
	}
	defer f.Close()

	daemonLogger := slog.New(slog.NewTextHandler(f, nil))
	fmt.Printf("🔄 Sync daemon started. Log: %s\n", syncLog)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if isVaultUnlocked() {
				daemonLogger.Info("Starting sync cycle")
				start := time.Now()
				vault.Save("")
				if err := pushVaultInternal(); err != nil {
					daemonLogger.Error("Sync failed", "error", err)
				} else {
					daemonLogger.Info("Sync completed", "elapsed", time.Since(start))
				}
			} else {
				daemonLogger.Debug("Vault locked, skipping sync")
			}
		case <-ctx.Done():
			daemonLogger.Info("Sync daemon stopping")
			return
		}
	}
}

func isVaultUnlocked() bool {
	if !utils.IsMounted(vault.MountPath) {
		return false
	}
	_, err := os.Stat(vault.PassCache)
	return err == nil
}

func pull() {
	subarg := ""
	if len(os.Args) > 2 {
		subarg = os.Args[2]
	}

	switch subarg {
	case "", "vault":
		pullVault()
	case "image":
		updateImage()
	default:
		logger.Error("Unknown pull target", "target", subarg)
	}
}

func updateImage() {
	if cfg.Image == "" {
		logger.Error("No image configured", "config", ConfigPath)
		return
	}

	fmt.Printf("🐳 Pulling image %s...\n", cfg.Image)
	cmd := execCommand("docker", "pull", cfg.Image)
	if err := cmd.Run(); err != nil {
		logger.Error("Image pull failed", "image", cfg.Image, "error", err)
		return
	}
	fmt.Println("✅ Image updated.")
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
		logger.Error("Unknown push target", "target", subarg)
	}
}

func pullVault() {
	loadVaultAWSCredentials()
	cwd, _ := os.Getwd()
	vaultFile := filepath.Join(cwd, ".tazpod", "vault", "vault.tar.aes")

	s3, err := utils.NewS3Client("", cfg.AwsSso.Profile)
	if err != nil {
		logger.Error("S3 Client error", "error", err)
		os.Exit(1)
	}

	os.MkdirAll(filepath.Join(cwd, ".tazpod", "vault"), 0755)
	fmt.Println("☁️  Downloading vault.tar.aes from S3...")
	if err := s3.DownloadFile("tazpod/vault/vault.tar.aes", vaultFile); err != nil {
		logger.Error("Download failed", "error", err)
		os.Exit(1)
	}
	fmt.Println("✅ Vault pulled. Run 'tazpod unlock' to decrypt.")
}

func pushVault() {
	start := time.Now()
	fmt.Println("☁️  Uploading vault.tar.aes to S3...")
	if err := pushVaultInternal(); err != nil {
		logger.Error("Push failed", "error", err)
		return
	}
	fmt.Printf("✅ Vault pushed successfully in %v.\n", time.Since(start))
}

func pushVaultInternal() error {
	loadVaultAWSCredentials()
	cwd, _ := os.Getwd()
	vaultFile := filepath.Join(cwd, ".tazpod", "vault", "vault.tar.aes")

	if _, err := os.Stat(vaultFile); os.IsNotExist(err) {
		return fmt.Errorf("no vault file found at %s", vaultFile)
	}

	s3, err := utils.NewS3Client("", cfg.AwsSso.Profile)
	if err != nil {
		return fmt.Errorf("S3 Client error: %w", err)
	}

	return s3.UploadFile("tazpod/vault/vault.tar.aes", vaultFile)
}
