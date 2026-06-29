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
	case "unlock":
		unlock()
	case "lock":
		lock()
	case "save":
		save()
	case "sync", "pull":
		pull()
	case "update":
		updateImage()
	case "push":
		push()
	case "login":
		login()
	case "list":
		list()
	case "vpn":
		if cfg.Mode == "lxc" {
			fmt.Println("⚠️  'vpn' is not available in LXC mode — Tailscale already provides the VPN tunnel.")
			return
		}
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
