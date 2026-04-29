package main

import (
	"fmt"
)

func help() {
	fmt.Printf("🛡️  TazPod CLI %s (RAM Vault)\n", Version)
	fmt.Println("\nUsage: tazpod <command> [arguments]")
	fmt.Println("\nLifecycle Commands:")
	fmt.Println("  init           Initialize a new TazPod project in the current directory")
	fmt.Println("  up             Start the development container")
	fmt.Println("  down           Stop and remove the development container")
	fmt.Println("  ssh | enter    Enter the container shell")
	fmt.Println("\nVault & Secrets Commands:")
	fmt.Println("  unlock         Unlock the RAM vault (Ghost Mode)")
	fmt.Println("  lock           Lock and wipe the RAM vault")
	fmt.Println("  save           Save the current RAM vault content to disk")
	fmt.Println("  login          Authenticate with AWS SSO")
	fmt.Println("  update             Pull the configured container image")
	fmt.Println("  pull [vault|image] Pull vault from S3 (default) or pull the configured image (alias: sync)")
	fmt.Println("  push [vault]       Push vault to S3 (default: vault)")
	fmt.Println("\nUtility Commands:")
	fmt.Println("  vpn up|down    Manage VPN connection for the active provider")
	fmt.Println("  setup-storage  Initialize S3 bucket")
	fmt.Println("  --version | -v Print version")
}
