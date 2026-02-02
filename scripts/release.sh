#!/bin/bash
# --- TAZPOD RELEASE AUTOMATOR ---
# This script simplifies the release process for the TazPod CLI.
# It compiles the Go binary, manages git tags, and uses the GitHub CLI (gh)
# to create a release and upload the binary artifact.

set -e

# --- CONFIGURATION ---
BINARY_NAME="tazpod"
REPO="tazzo/tazpod"

# --- COLORS ---
BLUE='\033[0;34m'
GREEN='\033[0;32m'
RED='\033[0;31m'
RESET='\033[0m'

# PREREQUISITE CHECK
# Ensure GitHub CLI is installed as it is required for creating releases.
if ! command -v gh &> /dev/null; then
    echo -e "${RED}❌ GitHub CLI (gh) not found. Please install it first.${RESET}"
    exit 1
fi

# 1. VERSION INPUT
# Retrieve the current git tag to display as context.
current_tag=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
echo -e "${BLUE}Current version: $current_tag${RESET}"

# Prompt the user for the new version number.
read -p "Enter new version (e.g. v1.0.0): " VERSION

if [[ -z "$VERSION" ]]; then
    echo -e "${RED}❌ Version cannot be empty.${RESET}"
    exit 1
fi

# 2. BUILD BINARY
# Compile the Go application for Linux AMD64 architecture.
# This ensures the released binary is compatible with the target deployment environment.
echo -e "${BLUE}🔨 Building TazPod for Linux/AMD64...${RESET}"
export GOOS=linux
export GOARCH=amd64
go build -o $BINARY_NAME cmd/tazpod/main.go

# 3. COMMIT AND TAG
# Stage changes, commit (if any), push to remote, and create a git tag.
echo -e "${BLUE}🏷️  Tagging and pushing code...${RESET}"
git add .
git commit -m "chore: release $VERSION" || echo "No changes to commit"
git push
git tag -a "$VERSION" -m "Release $VERSION"
git push origin "$VERSION"

# 4. GITHUB RELEASE
# Use the GitHub CLI to create a new release and upload the compiled binary.
echo -e "${BLUE}🚀 Creating GitHub Release...${RESET}"
gh release create "$VERSION" "$BINARY_NAME" \
    --repo "$REPO" \
    --title "Release $VERSION" \
    --notes "Automated release of TazPod $VERSION"

echo -e "${GREEN}✅ Successfully released $VERSION to GitHub!${RESET}"
echo -e "🔗 URL: https://github.com/$REPO/releases/tag/$VERSION"