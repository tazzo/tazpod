#!/bin/bash
set -e

CONTAINER_NAME="tazpod-lab"
IMAGE_NAME="tazpod-engine:local"
PROJECT_DIR=$(pwd)

# 1. Build Engine (Solo se necessario o forzato)
if [[ "$(docker images -q $IMAGE_NAME 2> /dev/null)" == "" ]] || [[ "$1" == "--rebuild" ]]; then
    echo "🏗️  Building TazPod Engine..."
    cd /home/taz/kubernetes/devpod
    docker build -f Dockerfile.base -t $IMAGE_NAME .
    cd $PROJECT_DIR
else
    echo "✅ Engine already ready."
fi

# 2. Cleanup old container
docker rm -f $CONTAINER_NAME 2>/dev/null || true

# 3. Launch
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