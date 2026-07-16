package main

import (
	"fmt"
)

func help() {
	fmt.Printf("🛡️  TazPod CLI %s (Gopass Store)\n", Version)
	fmt.Println("\nUsage: tazpod <command> [arguments]")
	fmt.Println("\nLifecycle Commands:")
	fmt.Println("  init           Initialize a new TazPod project in the current directory")
	fmt.Println("  up             Start the development container")
	fmt.Println("  down           Stop and remove the development container")
	fmt.Println("  ssh | enter    Enter the container shell")
	fmt.Println("  update         Pull the configured container image")
	fmt.Println("\nSecrets & Gopass Commands:")
	fmt.Println("  gopass         Interactive setup for the local gopass secrets store")
	fmt.Println("  lock           Kill GPG agent and revoke cached passphrases")
	fmt.Println("\nUtility Commands:")
	fmt.Println("  --version | -v Print version")
}
