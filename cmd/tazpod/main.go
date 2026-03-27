package main

import (
	"fmt"
	"log/slog"
	"os"
)

var logger *slog.Logger

func main() {
	var debug bool
	for _, arg := range os.Args {
		if arg == "--debug" {
			debug = true
			break
		}
	}

	loadConfigs()
	if cfg.Features.Debug {
		debug = true
	}

	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	if len(os.Args) < 2 {
		help()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "init":
		initProject()
	case "up":
		up()
	case "down":
		down()
	case "ssh", "enter":
		enter()
	case "unlock":
		unlock()
	case "lock":
		lock()
	case "save":
		save()
	case "sync", "pull":
		pull()
	case "push":
		push()
	case "login":
		login()
	case "vpn":
		vpnCommand()
	case "setup-storage":
		setupStorage()
	case "__internal_env":
		printExportEnv()
	case "__internal_sync_daemon":
		syncDaemon()
	case "--version", "-v":
		fmt.Println(Version)
	default:
		logger.Error("Unknown command", "command", command)
		os.Exit(1)
	}
}

func printExportEnv() {
	// Not implemented or placeholder
}
