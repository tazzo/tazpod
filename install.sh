#!/bin/bash
# --- TAZPOD INSTALLER ---
set -e

# Colors
BLUE='\033[0;34m'
GREEN='\033[0;32m'
RESET='\033[0m'

echo -e "${BLUE}🛡️ Starting TazPod Installation...${RESET}"

# 1. Determine OS and Arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
INSTALL_DIR="$HOME/.local/bin"

# 2. Ensure Install Dir exists
mkdir -p "$INSTALL_DIR"

# 3. Download (Simulated for now, would point to GitHub Releases)
# In our local lab, we just copy the compiled binary if we are in the repo
if [ -f "./tazpod" ]; then
    echo "📦 Local binary found, installing..."
    cp ./tazpod "$INSTALL_DIR/tazpod"
else
    echo "☁️ Binary not found locally. (In production, I would download from GitHub here)"
    # Example: curl -L https://github.com/tazzo/tazpod/releases/latest/download/tazpod-$OS-$ARCH -o "$INSTALL_DIR/tazpod"
fi

# 4. Set Permissions
chmod +x "$INSTALL_DIR/tazpod"

echo -e "${GREEN}✅ TazPod installed successfully in $INSTALL_DIR/tazpod${RESET}"

# 5. Check PATH
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    echo -e "\n⚠️  ${BLUE}$INSTALL_DIR${RESET} is not in your PATH."
    echo "Add this to your .bashrc or .zshrc:"
    echo -e "  export PATH=\$PATH:$INSTALL_DIR"
fi

echo -e "\n🚀 Run '${BLUE}tazpod init${RESET}' in your project directory to start!"

