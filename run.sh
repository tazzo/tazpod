#!/bin/bash
set -e

# --- TAZPOD RUNNER ---
# This script manages the lifecycle of the development container.

CONTAINER_NAME="tazpod-lab"
IMAGE_NAME="tazpod-engine:local"
PROJECT_DIR=$(pwd)

# 1. Build Engine Image (Only if missing or forced with --rebuild)
if [[ "$(docker images -q $IMAGE_NAME 2> /dev/null)" == "" ]] || [[ "$1" == "--rebuild" ]]; then
    echo "🏗️  Building TazPod Engine..."
    # We move to the devpod directory to ensure Docker finds Dockerfile.base and the compiled binary
    cd /home/taz/kubernetes/devpod
    docker build -f Dockerfile.base -t $IMAGE_NAME .
    cd $PROJECT_DIR
else
    echo "✅ Engine already ready."
fi

# 2. Cleanup existing container instances
docker rm -f $CONTAINER_NAME 2>/dev/null || true

# 3. Launch the privileged container
# --privileged is required for LUKS encryption and filesystem mounting
# --network host allows seamless integration with local Kubernetes clusters
echo "🚀 Starting TazPod in $PROJECT_DIR..."
docker run -d \
    --name $CONTAINER_NAME \
    --privileged \
    --network host \
    -v "$PROJECT_DIR:/workspace" \
    -w "/workspace" \
    $IMAGE_NAME \
    sleep infinity

echo "✅ Ready. Run '../devpod/shell.sh' to enter."
