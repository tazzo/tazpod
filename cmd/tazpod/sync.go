package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"tazpod/internal/utils"
	"tazpod/internal/vault"
)

func initLogger() {
	level := slog.LevelInfo
	switch os.Getenv("TAZPOD_LOG_LEVEL") {
	case "DEBUG":
		level = slog.LevelDebug
	case "WARN":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: level}
	logger = slog.New(slog.NewTextHandler(os.Stderr, opts))
}

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
			start := time.Now()
			if isVaultUnlocked() {
				vault.Save("")
			}
			if err := pushVaultInternal(readContentHash()); err != nil {
				daemonLogger.Error("Sync failed", "error", err)
			} else {
				daemonLogger.Info("Sync completed", "elapsed", time.Since(start))
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
	index := 0
	if len(os.Args) > 2 {
		subarg = os.Args[2]
	}
	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--index" && i+1 < len(os.Args) {
			index, _ = strconv.Atoi(os.Args[i+1])
		}
	}
	switch subarg {
	case "vault", "":
		if index == 0 {
			pullVault()
		} else {
			pullVaultByIndex(index)
		}
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

// vaultFilePath prova path container e host, restituisce il primo che esiste.
func vaultFilePath() string {
	paths := []string{
		vault.VaultFile,
		filepath.Join(".tazpod", "vault", "vault.tar.aes"),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return paths[0]
}

// readContentHash legge l'hash dal file sidecar last-content.hash.
func readContentHash() string {
	data, err := os.ReadFile(filepath.Join(filepath.Dir(vaultFilePath()), "last-content.hash"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func pullVault() {
	loadVaultAWSCredentials()
	vaultFile := vaultFilePath()

	s3, err := utils.NewS3Client(cfg.AwsSso.Bucket, cfg.AwsSso.Region, cfg.AwsSso.Profile)
	if err != nil {
		logger.Error("S3 Client error", "error", err)
		os.Exit(1)
	}

	os.MkdirAll(filepath.Dir(vaultFile), 0755)
	fmt.Println("☁️  Downloading vault.tar.aes from S3...")
	if err := s3.DownloadFile("tazpod/vault/vault.tar.aes", vaultFile); err != nil {
		logger.Error("Download failed", "error", err)
		os.Exit(1)
	}
	fmt.Println("✅ Vault pulled. Run 'tazpod unlock' to decrypt.")
}

func pullVaultByIndex(index int) {
	loadVaultAWSCredentials()
	dest := vaultFilePath()

	s3, err := utils.NewS3Client(cfg.AwsSso.Bucket, cfg.AwsSso.Region, cfg.AwsSso.Profile)
	if err != nil {
		logger.Error("S3 Client error", "error", err)
		os.Exit(1)
	}

	objects, err := s3.ListObjects("tazpod/vault/history/")
	if err != nil {
		logger.Error("List failed", "error", err)
		os.Exit(1)
	}

	sort.Slice(objects, func(i, j int) bool {
		return objects[i].LastModified.After(objects[j].LastModified)
	})
	if index-1 >= len(objects) {
		logger.Error("Only %d copies available", "count", len(objects))
		os.Exit(1)
	}

	os.MkdirAll(filepath.Dir(dest), 0755)
	if err := s3.DownloadFile(objects[index-1].Key, dest); err != nil {
		logger.Error("Download failed", "error", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Pulled (index %d)\n", index)
}

func pushVault() {
	start := time.Now()
	fmt.Println("☁️  Uploading vault to S3...")
	contentHash := readContentHash()
	if err := pushVaultInternal(contentHash); err != nil {
		logger.Error("Push failed", "error", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Vault pushed successfully in %v.\n", time.Since(start))
}

func pushVaultInternal(contentHash string) error {
	loadVaultAWSCredentials()
	vaultFile := vaultFilePath()

	if _, err := os.Stat(vaultFile); os.IsNotExist(err) {
		return fmt.Errorf("no vault file found at %s", vaultFile)
	}

	s3, err := utils.NewS3Client(cfg.AwsSso.Bucket, cfg.AwsSso.Region, cfg.AwsSso.Profile)
	if err != nil {
		return fmt.Errorf("S3 Client error: %w", err)
	}

	// Hash skip — solo se contentHash non è vuoto (sidecar presente)
	if contentHash != "" {
		lastMeta, headErr := s3.HeadObject("tazpod/vault/vault.tar.aes")
		if headErr == nil {
			if lastHash, ok := lastMeta["content-sha256"]; ok && lastHash == contentHash {
				slog.Info("Vault unchanged, skipping push")
				return nil
			}
		} else {
			slog.Info("HeadObject failed (first push or transient)", "error", headErr)
		}
	}

	timestamp := time.Now().UTC().Format("20060102T150405")
	meta := map[string]string{"content-sha256": contentHash}

	if err := s3.UploadFileWithMetadata("tazpod/vault/history/vault-"+timestamp+".tar.aes", vaultFile, meta); err != nil {
		return fmt.Errorf("history upload: %w", err)
	}

	if err := s3.UploadFileWithMetadata("tazpod/vault/vault.tar.aes", vaultFile, meta); err != nil {
		return fmt.Errorf("latest upload: %w", err)
	}

	pruneHistory(s3, cfg.Vault.Retention)
	return nil
}

func listVaultHistory() {
	loadVaultAWSCredentials()
	s3, err := utils.NewS3Client(cfg.AwsSso.Bucket, cfg.AwsSso.Region, cfg.AwsSso.Profile)
	if err != nil {
		logger.Error("S3 Client error", "error", err)
		os.Exit(1)
	}

	fmt.Printf("  %-6s  %-26s  %-8s\n", "Index", "Timestamp", "Size")
	fmt.Printf("  %-6d  %-26s  %-8s\n", 0, "latest", "—")

	objects, err := s3.ListObjects("tazpod/vault/history/")
	if err != nil {
		fmt.Println("  (no history yet)")
		return
	}

	sort.Slice(objects, func(i, j int) bool {
		return objects[i].LastModified.After(objects[j].LastModified)
	})
	for i, obj := range objects {
		fmt.Printf("  %-6d  %-26s  %-8d\n",
			i+1, obj.LastModified.Format("2006-01-02 15:04:05"), obj.Size)
	}
}

func pruneHistory(s3 *utils.S3Client, maxCopies int) {
	objects, err := s3.ListObjects("tazpod/vault/history/")
	if err != nil {
		slog.Warn("Prune: list failed", "error", err)
		return
	}
	if len(objects) <= maxCopies {
		return
	}

	sort.Slice(objects, func(i, j int) bool {
		return objects[i].LastModified.Before(objects[j].LastModified)
	})
	toDelete := objects[:len(objects)-maxCopies]
	keys := make([]string, len(toDelete))
	for i, obj := range toDelete {
		keys[i] = obj.Key
	}

	if err := s3.DeleteObjects(keys); err != nil {
		slog.Warn("Prune: delete failed", "error", err)
		return
	}
	slog.Info("Pruned %d old history copies", "count", len(keys))
}
