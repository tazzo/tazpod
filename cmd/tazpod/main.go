package main

import (
	"fmt"
	"log/slog"
	"os"
)

var logger *slog.Logger

func main() {
	loadConfigs()
	initLogger()

	// Sovrascrive level da config o --debug per retrocompatibilità
	var debug bool
	for _, arg := range os.Args {
		if arg == "--debug" {
			debug = true
			break
		}
	}
	if cfg.Features.Debug {
		debug = true
	}
	if debug {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	// Estrai --mode flag prima del dispatch
	var initMode string
	args := []string{}
	skipNext := false
	for i, a := range os.Args[1:] {
		if skipNext {
			skipNext = false
			continue
		}
		if a == "--mode" && i+2 < len(os.Args) {
			initMode = os.Args[i+2]
			skipNext = true
			continue
		}
		args = append(args, a)
	}

	if len(args) < 1 {
		smartEntry()
		return
	}

	command := args[0]

	switch command {
	case "help", "--help", "-h":
		help()
	case "init":
		initProject(initMode)
	case "up":
		up()
	case "down":
		down()
	case "ssh", "enter":
		enter()
	case "lock":
		lock()
	case "gopass":
		gopassCmd()
	case "update":
		updateImage()
	case "unlock":
		fmt.Println("⚠️  'unlock' is deprecated — secrets are now managed via gopass inside the container.")
		fmt.Println("   Run 'tazpod gopass' to configure the store.")
	case "save":
		fmt.Println("⚠️  'save' is deprecated — secrets are now managed via gopass inside the container.")
		fmt.Println("   Any changes are persisted instantly to the git repository.")
	case "sync", "pull", "push", "list":
		fmt.Printf("⚠️  '%s' is deprecated — secrets are managed locally in gopass and versioned via git.\n", command)
	case "login":
		fmt.Println("⚠️  'login' is deprecated — S3 sync and AWS SSO are no longer required.")
	case "vpn":
		fmt.Println("⚠️  'vpn' (WireGuard) is deprecated. Tailscale is automatically started inside the container to provide the VPN tunnel.")
	case "setup-storage":
		fmt.Println("⚠️  'setup-storage' is deprecated — S3 backup is decommissioned.")
	case "__internal_sync_daemon":
		fmt.Println("⚠️  'sync daemon' is deprecated.")
	case "--version", "-v":
		fmt.Println(Version)
	default:
		logger.Error("Unknown command", "command", command)
		os.Exit(1)
	}
}

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
