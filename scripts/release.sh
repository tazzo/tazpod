#!/bin/bash
# --- TAZPOD RELEASE AUTOMATOR ---
set -e

# Configuration
BINARY_NAME="tazpod"
REPO="tazzo/tazpod"

# Colors
BLUE='\033[0;34m'
GREEN='\033[0;32m'
RED='\033[0;31m'
RESET='\033[0m'

# Check for GitHub CLI
if ! command -v gh &> /dev/null; then
    echo -e "${RED}❌ GitHub CLI (gh) not found. Please install it first.${RESET}"
    exit 1
fi

# 1. Ask for version
current_tag=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
echo -e "${BLUE}Current version: $current_tag${RESET}"
read -p "Enter new version (e.g. v1.0.0): " VERSION

if [[ -z "$VERSION" ]]; then
    echo -e "${RED}❌ Version cannot be empty.${RESET}"
    exit 1
fi

# 2. Build Binary
echo -e "${BLUE}🔨 Building TazPod for Linux/AMD64...${RESET}"
export GOOS=linux
export GOARCH=amd64
go build -o $BINARY_NAME cmd/tazpod/main.go

# 3. Git Tag and Push
echo -e "${BLUE}🏷️  Tagging and pushing code...${RESET}"
git add .
git commit -m "chore: release $VERSION" || echo "No changes to commit"
git push
git tag -a "$VERSION" -m "Release $VERSION"
git push origin "$VERSION"

# 4. Create GitHub Release and Upload Asset
echo -e "${BLUE}🚀 Creating GitHub Release...${RESET}"
gh release create "$VERSION" "$BINARY_NAME" \
    --repo "$REPO" \
    --title "Release $VERSION" \
    --notes "Automated release of TazPod $VERSION"

echo -e "${GREEN}✅ Successfully released $VERSION to GitHub!${RESET}"
echo -e "🔗 URL: https://github.com/$REPO/releases/tag/$VERSION"
