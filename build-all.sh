#!/bin/bash
# --- TAZPOD UNIFIED BUILDER (LOCAL ONLY) ---
# Lancio: bash build-all.sh

set -e

echo "🏗️  Inizio compilazione TazPod Atomic Layers..."
echo "--------------------------------------------------"

# 1. BASE LAYER
echo "📦 [1/4] Build: TazPod Base (Python, Security, Core)"
docker build -t tazzo/tazlab.net:tazpod-base -f .tazpod/Dockerfile.base .

# 2. INFISICAL LAYER
echo "📦 [2/4] Build: TazPod Infisical"
docker build -t tazzo/tazlab.net:tazpod-infisical -f .tazpod/Dockerfile.infisical .

# 3. K8S LAYER
echo "📦 [3/4] Build: TazPod K8s (Ops Tools)"
docker build -t tazzo/tazlab.net:tazpod-k8s -f .tazpod/Dockerfile.k8s .

# 4. GEMINI LAYER
echo "📦 [4/4] Build: TazPod Gemini (AI Ready)"
docker build -t tazzo/tazlab.net:tazpod-gemini -f .tazpod/Dockerfile.gemini .

echo "--------------------------------------------------"
echo "✅ TUTTI I LIVELLI SONO STATI COMPILATI LOCALMENTE!"
echo "➡️  Puoi lanciare il TazPod aggiornato con 'tazpod up'."
