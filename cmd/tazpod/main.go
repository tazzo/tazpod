package main

import (
	"fmt"
	"os"
	"tazpod/internal/engine"
	"tazpod/internal/utils"
	"tazpod/internal/vault"
)

func main() {
	if len(os.Args) < 2 {
		help()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	// Host Commands
	case "up":
		engine.Up()
	case "down":
		engine.Down()
	case "enter", "ssh":
		engine.Enter()

	// Container Commands
	case "unlock":
		checkInside()
		vault.Unlock()
	case "lock":
		checkInside()
		vault.Lock()
	case "reinit":
		checkInside()
		vault.Reinit()
	case "env":
		checkInside()
		vault.ExportEnv()

	// Internal Command
	case "internal-ghost":
		vault.InternalGhost()

	default:
		help()
		os.Exit(1)
	}
}

func help() {
	fmt.Println("Usage: tazpod <command>")
	fmt.Println("  up      -> Build & Start TazPod container (Host)")
	fmt.Println("  down    -> Stop & Remove TazPod container (Host)")
	fmt.Println("  ssh     -> Enter TazPod container (Host)")
	fmt.Println("  unlock  -> Unlock Vault & Start Secure Shell (Container)")
	fmt.Println("  lock    -> Close Vault & Stay in Container (Container)")
	fmt.Println("  reinit  -> Wipe Vault & Start Fresh (Container)")
}

func checkInside() {
	if !utils.CheckInside() {
		fmt.Println("❌ This command must be run INSIDE the TazPod container.")
		os.Exit(1)
	}
}
