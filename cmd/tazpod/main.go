package main

import (
	"fmt"
	"os"
)

func main() {
	loadConfigs()

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
		fmt.Fprintf(os.Stderr, "❌ Unknown command: %s\n", command)
		os.Exit(1)
	}
}

func printExportEnv() {
	// Not implemented or placeholder
}
