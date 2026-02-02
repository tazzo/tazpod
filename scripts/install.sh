#!/bin/bash
# --- TAZPOD UNIVERSAL INSTALLER ---
# This script automates the installation of the TazPod CLI tool.
# It detects the operating system and architecture, downloads the correct binary,
# and ensures it is executable and in the user's PATH.

set -e

# --- CONFIGURATION ---
REPO="tazzo/tazpod"
INSTALL_DIR="$HOME/.local/bin"

# --- COLORS FOR OUTPUT ---
BLUE='\033[0;34m'
GREEN='\033[0;32m'
RED='\033[0;31m'
RESET='\033[0m'

echo -e "${BLUE}🛡️  TazPod Installer starting...${RESET}"

# 1. DETECT OS AND ARCHITECTURE
# Identify the kernel name (e.g., Linux, Darwin) and normalize to lowercase.
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Normalize architecture names to standard conventions (amd64, arm64).
if [ "$ARCH" == "x86_64" ]; then
    ARCH="amd64"
elif [ "$ARCH" == "aarch64" ] || [ "$ARCH" == "arm64" ]; then
    ARCH="arm64"
fi

echo -e "🔎 Detected: $OS-$ARCH"

# 2. PREPARE INSTALLATION DIRECTORY
# Create the target directory if it doesn't already exist.
mkdir -p "$INSTALL_DIR"

# 3. DOWNLOAD BINARY
# Fetch the 'tazpod' binary from the latest release of the GitHub repository.
# Note: Currently assumes a single binary name 'tazpod'. Future versions might require dynamic naming based on OS/Arch.
BINARY_URL="https://github.com/$REPO/releases/latest/download/tazpod"

echo -e "📥 Downloading TazPod from GitHub..."
if ! curl -L "$BINARY_URL" -o "$INSTALL_DIR/tazpod"; then
    echo -e "${RED}❌ Download failed. Make sure you have created a 'latest' release on GitHub with the 'tazpod' binary attached.${RESET}"
    exit 1
fi

# 4. SET PERMISSIONS
# Make the downloaded binary executable.
chmod +x "$INSTALL_DIR/tazpod"

echo -e "${GREEN}✅ TazPod installed successfully in $INSTALL_DIR/tazpod${RESET}"

# 5. PATH VERIFICATION
# Check if the installation directory is currently in the user's system PATH.
# If not, advise the user to add it to their shell configuration file.
if [[ ":$PATH:" != ":$INSTALL_DIR:"* ]]; then
    echo -e "\n⚠️  ${BLUE}$INSTALL_DIR${RESET} is not in your PATH."
    echo "Add this to your .bashrc or .zshrc:"
    echo -e "  export PATH=\$PATH:$INSTALL_DIR"
fi

echo -e "\n🚀 Run '${BLUE}tazpod init${RESET}' in your project directory to start!"
