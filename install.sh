#!/bin/bash
set -e

# lolcathost installer script
# Usage: curl -fsSL https://raw.githubusercontent.com/lukaszraczylo/lolcathost/main/install.sh | bash

REPO="lukaszraczylo/lolcathost"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="lolcathost"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$OS" in
        darwin)
            OS="darwin"
            ;;
        linux)
            OS="linux"
            ;;
        *)
            error "Unsupported operating system: $OS"
            ;;
    esac

    case "$ARCH" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        arm64|aarch64)
            ARCH="arm64"
            ;;
        *)
            error "Unsupported architecture: $ARCH"
            ;;
    esac

    PLATFORM="${OS}_${ARCH}"
    info "Detected platform: $PLATFORM"
}

# Get latest release version
get_latest_version() {
    info "Fetching latest release..."
    VERSION=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

    if [ -z "$VERSION" ]; then
        error "Failed to fetch latest version"
    fi

    info "Latest version: $VERSION"
}

# Download and install
download_and_install() {
    # Strip 'v' prefix from version for filename (goreleaser uses version without v)
    VERSION_NUM=${VERSION#v}

    # Construct download URL (matches goreleaser naming: lolcathost-VERSION-OS-ARCH.tar.gz)
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}-${VERSION_NUM}-${OS}-${ARCH}.tar.gz"

    info "Downloading from: $DOWNLOAD_URL"

    # Create temp directory
    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT

    # Download
    if ! curl -sL "$DOWNLOAD_URL" -o "$TMP_DIR/lolcathost.tar.gz"; then
        error "Failed to download release"
    fi

    # Extract
    info "Extracting..."
    tar -xzf "$TMP_DIR/lolcathost.tar.gz" -C "$TMP_DIR"

    # Find the binary (might be in root or subdirectory)
    BINARY_PATH=$(find "$TMP_DIR" -name "$BINARY_NAME" -type f | head -1)

    if [ -z "$BINARY_PATH" ]; then
        error "Binary not found in archive"
    fi

    # Install
    info "Installing to $INSTALL_DIR/$BINARY_NAME..."

    if [ -w "$INSTALL_DIR" ]; then
        cp "$BINARY_PATH" "$INSTALL_DIR/$BINARY_NAME"
        chmod +x "$INSTALL_DIR/$BINARY_NAME"
    else
        warn "Need sudo to install to $INSTALL_DIR"
        sudo cp "$BINARY_PATH" "$INSTALL_DIR/$BINARY_NAME"
        sudo chmod +x "$INSTALL_DIR/$BINARY_NAME"
    fi

    info "Binary installed successfully!"
}

# Main
main() {
    echo ""
    echo "  lolcathost installer"
    echo "  ===================="
    echo ""

    detect_platform
    get_latest_version
    download_and_install

    echo ""
    info "Installation complete!"
    echo ""
    echo "  Next steps:"
    echo "    1. Install the daemon:  sudo lolcathost --install"
    echo "    2. Open a new terminal (for group membership)"
    echo "    3. Run:                 lolcathost"
    echo ""
}

main
