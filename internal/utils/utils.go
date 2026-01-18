package utils

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// RunCmd executes a command and streams stdout/stderr to the console
func RunCmd(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.Run()
}

// RunOutput executes a command and returns its trimmed stdout
func RunOutput(name string, args ...string) string {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// RunWithStdin executes a command feeding it a string as stdin
func RunWithStdin(input, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = bytes.NewBufferString(input)
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	err := cmd.Run()
	if err != nil {
		fmt.Printf("\n❌ SYSTEM ERROR [%s]: %s\n", name, stderr.String())
	}
	return out.String(), err
}

// FileExist returns true if a file or directory exists
func FileExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsMounted checks if a given path is currently a mount point
func IsMounted(path string) bool {
	out, _ := exec.Command("mount").Output()
	return strings.Contains(string(out), path)
}

// WaitForDevice waits up to 4 seconds for a device node to appear
func WaitForDevice(path string) {
	for i := 0; i < 20; i++ {
		if FileExist(path) {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// CheckInside checks if we are running inside a container
func CheckInside() bool {
	_, err := os.Stat("/.dockerenv")
	return !os.IsNotExist(err)
}
