#!/bin/bash
set -e

# --- TAZPOD BASE PUBLISHER ---
IMAGE_NAME="tazzo/tazlab.net:tazpod-base"
LOCAL_NAME="tazpod-engine:local"

echo "🏗️  Step 1: Building local image..."
# Usiamo il binario Go per garantire che la build locale sia sincronizzata
./tazpod up

echo "🏷️  Step 2: Tagging for Docker Hub..."
docker tag $LOCAL_NAME $IMAGE_NAME

echo "🚀 Step 3: Pushing to Cloud..."
docker push $IMAGE_NAME

echo "✅ TazPod Base is now online: $IMAGE_NAME"
