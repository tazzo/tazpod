package main

import (
	"fmt"
	"os"
	"os/exec"
)

func vpnCommand() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: tazpod vpn up|down")
		os.Exit(1)
	}

	action := os.Args[2]
	switch action {
	case "up":
		vpnUp()
	case "down":
		vpnDown()
	default:
		logger.Error("Unknown VPN action", "action", action)
		os.Exit(1)
	}
}

func vpnUp() {
	fmt.Println("🔌 Starting VPN tunnel...")
	// Logic for WireGuard or other providers goes here.
	// For now, shells out to a script placeholder.
	cmd := exec.Command("sudo", "wg-quick", "up", "wg0")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		logger.Error("VPN start failed", "error", err)
		return
	}
	fmt.Println("✅ VPN tunnel active.")
}

func vpnDown() {
	fmt.Println("🔌 Stopping VPN tunnel...")
	cmd := exec.Command("sudo", "wg-quick", "down", "wg0")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		logger.Error("VPN stop failed", "error", err)
		return
	}
	fmt.Println("✅ VPN tunnel stopped.")
}
