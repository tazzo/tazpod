#!/bin/bash
set -e

# --- TAZPOD MULTI-LAYER PUBLISHER ---
# This script handles the building and publishing of the TazPod Docker image hierarchy.
# It builds images in a specific order (Base -> Infisical -> K8s -> Gemini) to ensure
# layer dependencies are correctly respected, and then pushes them to the registry.

# Usage: ./scripts/publish-base.sh [version]
# Example: ./scripts/publish-base.sh v1.0.0

VERSION=$1

# --- IMAGE DEFINITIONS ---
REPO_PREFIX="tazzo/tazlab.net"
BASE_NAME="tazpod-base"
INFISICAL_NAME="tazpod-infisical"
K8S_NAME="tazpod-k8s"
GEMINI_NAME="tazpod-gemini"

# Function to build and push
build_and_push() {
    NAME=$1
    DOCKERFILE=$2
    FULL_IMAGE="$REPO_PREFIX:$NAME"
    
    echo "🏗️  Building $NAME..."
    docker build -t "$FULL_IMAGE" -f "$DOCKERFILE" .
    
    if [ -n "$VERSION" ]; then
        VERSIONED_IMAGE="$REPO_PREFIX:$NAME-$VERSION"
        echo "🏷️  Tagging as $NAME-$VERSION..."
        docker tag "$FULL_IMAGE" "$VERSIONED_IMAGE"
    fi
}

push_image() {
    NAME=$1
    FULL_IMAGE="$REPO_PREFIX:$NAME"
    
    echo "🚀 Pushing $NAME..."
    docker push "$FULL_IMAGE"
    
    if [ -n "$VERSION" ]; then
        VERSIONED_IMAGE="$REPO_PREFIX:$NAME-$VERSION"
        echo "🚀 Pushing $NAME-$VERSION..."
        docker push "$VERSIONED_IMAGE"
    fi
}

# 1. BUILD
build_and_push $BASE_NAME ".tazpod/Dockerfile.base"
build_and_push $INFISICAL_NAME ".tazpod/Dockerfile.infisical"
build_and_push $K8S_NAME ".tazpod/Dockerfile.k8s"
build_and_push $GEMINI_NAME ".tazpod/Dockerfile.gemini"

# 2. PUSH
push_image $BASE_NAME
push_image $INFISICAL_NAME
push_image $K8S_NAME
push_image $GEMINI_NAME

echo "✅ All TazPod layers are now online."
if [ -n "$VERSION" ]; then
    echo "📦 Published version: $VERSION"
fi
