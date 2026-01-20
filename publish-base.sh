#!/bin/bash
set -e

# --- TAZPOD MULTI-LAYER PUBLISHER ---
BASE_IMAGE="tazzo/tazlab.net:tazpod-base"
INFISICAL_IMAGE="tazzo/tazlab.net:tazpod-infisical"
K8S_IMAGE="tazzo/tazlab.net:tazpod-k8s"

echo "🏗️  Step 1: Building Base..."
docker build -t $BASE_IMAGE -f .tazpod/Dockerfile.base .

echo "🏗️  Step 2: Building Infisical..."
docker build -t $INFISICAL_IMAGE -f .tazpod/Dockerfile.infisical .

echo "🏗️  Step 3: Building K8s..."
docker build -t $K8S_IMAGE -f .tazpod/Dockerfile.k8s .

echo "🚀 Step 4: Pushing to Docker Hub..."
docker push $BASE_IMAGE
docker push $INFISICAL_IMAGE
docker push $K8S_IMAGE

echo "✅ All TazPod layers (Base, Infisical, K8s) are now online."
