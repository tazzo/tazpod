#!/bin/bash
set -e

# Colori per output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}🚧 Inizio procedura di Rebuild Locale (Nuclear Option)...${NC}"

# 1. Spostati nella directory dello script (root di devpod)
cd "$(dirname "$0")"

# 2. Rebuild Base Image con Cache Busting
echo -e "${GREEN}1. Costruzione devpod-base:local (forcing dotfiles update)...${NC}"
docker build --build-arg CACHEBUST=$(date +%s) -f Dockerfile.base -t devpod-base:local .

# 3. Pulizia Immagini DevPod Cache
echo -e "${GREEN}2. Pulizia cache immagini DevPod (vsc-*)...${NC}"
# Trova immagini che iniziano con vsc-tazlab o vsc-devpod e cancellale
IMAGES=$(docker images -q "vsc-tazlab*" "vsc-devpod*")
if [ -n "$IMAGES" ]; then
    echo "$IMAGES" | xargs -r docker rmi -f
    echo "   ✅ Cache pulita."
else
    echo "   ✨ Nessuna immagine cache trovata (pulito)."
fi

echo -e "${GREEN}3. Pronto!${NC}"
echo -e "${YELLOW}👉 Ora vai nel tuo progetto e lancia:${NC}"
echo -e "   cd ../tazlab-k8s"
echo -e "   devpod delete . && devpod up . --ide none"
