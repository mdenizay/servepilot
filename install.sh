#!/bin/bash
# ╔═══════════════════════════════════════════════════════════════╗
# ║              ServePilot - Install Script                      ║
# ║      Self-Hosted Server Management CLI (Free Forge)           ║
# ╚═══════════════════════════════════════════════════════════════╝

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${CYAN}"
echo "╔═══════════════════════════════════════════════════════════════╗"
echo "║              ServePilot Installer v1.0.0                      ║"
echo "╚═══════════════════════════════════════════════════════════════╝"
echo -e "${NC}"

# Check root
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}Please run as root: sudo bash install.sh${NC}"
    exit 1
fi

# Check OS
if [ ! -f /etc/lsb-release ] && [ ! -f /etc/debian_version ]; then
    echo -e "${RED}This installer only supports Ubuntu/Debian${NC}"
    exit 1
fi

# Check if Go is installed, install if not
if ! command -v go &> /dev/null; then
    echo -e "${YELLOW}Installing Go...${NC}"
    GO_VERSION="1.22.5"
    ARCH=$(dpkg --print-architecture)
    wget -q "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz" -O /tmp/go.tar.gz
    rm -rf /usr/local/go
    tar -C /usr/local -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz
    export PATH=$PATH:/usr/local/go/bin
    echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile.d/go.sh
    echo -e "${GREEN}Go ${GO_VERSION} installed${NC}"
fi

# Build ServePilot
echo -e "${YELLOW}Building ServePilot...${NC}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

go mod tidy
go build -ldflags="-s -w" -o /usr/local/bin/servepilot .
chmod +x /usr/local/bin/servepilot

echo -e "${GREEN}ServePilot installed to /usr/local/bin/servepilot${NC}"
echo ""
echo -e "${CYAN}Quick start:${NC}"
echo "  servepilot init                                        # Initialize server"
echo "  servepilot site add --domain example.com --type laravel  # Add a Laravel site"
echo "  servepilot site add --domain app.com --type nextjs       # Add a Next.js site"
echo "  servepilot ssl issue --domain example.com                # Issue SSL certificate"
echo "  servepilot deploy setup --domain example.com --repo git@github.com:user/repo.git"
echo ""
echo -e "${GREEN}Run 'servepilot help' for all commands.${NC}"
