package engine

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"tazpod/internal/utils"
)

const (
	ContainerName = "tazpod-lab"
	ImageName     = "tazpod-engine:local"
	Dockerfile    = "Dockerfile.base"
)

// Up builds the image and launches the privileged container
func Up() {
	fmt.Println("🏗️  Ensuring TazPod Image (Compatible)...")
	utils.RunCmd("docker", "build", "-f", Dockerfile, "-t", ImageName, ".")

	fmt.Println("🛑 Cleaning instances...")
	exec.Command("docker", "rm", "-f", ContainerName).Run()

	cwd, _ := os.Getwd()
	fmt.Printf("🚀 Starting TazPod in %s...\n", cwd)

	display := os.Getenv("DISPLAY")
	xauth := os.Getenv("XAUTHORITY")
	if xauth == "" {
		xauth = os.Getenv("HOME") + "/.Xauthority"
	}

	utils.RunCmd("docker", "run", "-d",
		"--name", ContainerName,
		"--privileged",
		"--network", "host",
		"-e", "DISPLAY="+display,
		"-e", "XAUTHORITY=/home/tazpod/.Xauthority",
		"-v", "/tmp/.X11-unix:/tmp/.X11-unix",
		"-v", xauth+":/home/tazpod/.Xauthority",
		"-v", cwd+":/workspace",
		"-w", "/workspace",
		ImageName, "sleep", "infinity")

	fmt.Println("✅ Ready. Run './tazpod enter' to get inside.")
}

// Down stops and removes the container
func Down() {
	fmt.Println("🧹 Shutting down TazPod...")
	utils.RunCmd("docker", "rm", "-f", ContainerName)
	fmt.Println("✅ Done.")
}

// Enter performs a docker exec into the running container
func Enter() {
	binary, _ := exec.LookPath("docker")
	args := []string{"docker", "exec", "-it", ContainerName, "bash"}
	syscall.Exec(binary, args, os.Environ())
}
