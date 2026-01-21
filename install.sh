#!/bin/bash
# --- TAZPOD UNIVERSAL INSTALLER ---
set -e

# Configuration
REPO="tazzo/tazpod"
INSTALL_DIR="$HOME/.local/bin"

# Colors
BLUE='\033[0;34m'
GREEN='\033[0;32m'
RED='\033[0;31m'
RESET='\033[0m'

echo -e "${BLUE}🛡️  TazPod Installer starting...${RESET}"

# 1. Detect OS and Architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Map architecture names
if [ "$ARCH" == "x86_64" ]; then
    ARCH="amd64"
elif [ "$ARCH" == "aarch64" ] || [ "$ARCH" == "arm64" ]; then
    ARCH="arm64"
fi

echo -e "🔎 Detected: $OS-$ARCH"

# 2. Ensure Install Dir
mkdir -p "$INSTALL_DIR"

# 3. Download Latest Binary from GitHub Releases
# Note: For now, we point to a generic 'tazpod' name in the latest release
BINARY_URL="https://github.com/$REPO/releases/latest/download/tazpod"

echo -e "📥 Downloading TazPod from GitHub..."
if ! curl -L "$BINARY_URL" -o "$INSTALL_DIR/tazpod"; then
    echo -e "${RED}❌ Download failed. Make sure you have created a 'latest' release on GitHub with the 'tazpod' binary attached.${RESET}"
    exit 1
fi

# 4. Permissions
chmod +x "$INSTALL_DIR/tazpod"

echo -e "${GREEN}✅ TazPod installed successfully in $INSTALL_DIR/tazpod${RESET}"

# 5. PATH check
if [[ ":$PATH:" != ":$INSTALL_DIR:"* ]]; then
    echo -e "\n⚠️  ${BLUE}$INSTALL_DIR${RESET} is not in your PATH."
    echo "Add this to your .bashrc or .zshrc:"
    echo -e "  export PATH=\$PATH:$INSTALL_DIR"
fi

echo -e "\n🚀 Run '${BLUE}tazpod init${RESET}' in your project directory to start!"