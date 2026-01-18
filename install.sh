#!/bin/bash
set -euo pipefail

# ribbin installer
# Usage: curl -fsSL https://raw.githubusercontent.com/happycollision/ribbin/main/install.sh | bash

REPO="happycollision/ribbin"
INSTALL_DIR="${HOME}/.local/bin"
BINARY_NAME="ribbin"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() {
    echo -e "${GREEN}==>${NC} $1"
}

warn() {
    echo -e "${YELLOW}Warning:${NC} $1"
}

error() {
    echo -e "${RED}Error:${NC} $1" >&2
    exit 1
}

# Detect OS
detect_os() {
    local os
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    case "$os" in
        linux) echo "linux" ;;
        darwin) echo "darwin" ;;
        *) error "Unsupported operating system: $os" ;;
    esac
}

# Detect architecture
detect_arch() {
    local arch
    arch=$(uname -m)
    case "$arch" in
        x86_64|amd64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *) error "Unsupported architecture: $arch" ;;
    esac
}

# Get latest version from GitHub
get_latest_version() {
    local version
    version=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [[ -z "$version" ]]; then
        error "Failed to get latest version from GitHub"
    fi
    echo "$version"
}

# Download and verify binary
download_binary() {
    local version="$1"
    local os="$2"
    local arch="$3"
    local tmpdir

    tmpdir=$(mktemp -d)
    trap "rm -rf $tmpdir" EXIT

    local archive_name="${BINARY_NAME}_${version#v}_${os}_${arch}.tar.gz"
    local download_url="https://github.com/${REPO}/releases/download/${version}/${archive_name}"
    local checksums_url="https://github.com/${REPO}/releases/download/${version}/checksums.txt"

    info "Downloading ${BINARY_NAME} ${version} for ${os}/${arch}..."

    # Download archive
    if ! curl -fsSL -o "${tmpdir}/${archive_name}" "$download_url"; then
        error "Failed to download ${archive_name}"
    fi

    # Download checksums
    if ! curl -fsSL -o "${tmpdir}/checksums.txt" "$checksums_url"; then
        error "Failed to download checksums"
    fi

    # Verify checksum
    info "Verifying checksum..."
    cd "$tmpdir"
    if command -v sha256sum &> /dev/null; then
        grep "$archive_name" checksums.txt | sha256sum -c - > /dev/null 2>&1 || error "Checksum verification failed"
    elif command -v shasum &> /dev/null; then
        grep "$archive_name" checksums.txt | shasum -a 256 -c - > /dev/null 2>&1 || error "Checksum verification failed"
    else
        warn "Neither sha256sum nor shasum found, skipping checksum verification"
    fi

    # Extract archive
    info "Extracting..."
    tar -xzf "$archive_name"

    # Install binary
    info "Installing to ${INSTALL_DIR}..."
    mkdir -p "$INSTALL_DIR"
    mv "$BINARY_NAME" "${INSTALL_DIR}/${BINARY_NAME}"
    chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
}

# Check if install dir is in PATH
check_path() {
    if [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
        warn "${INSTALL_DIR} is not in your PATH"
        echo ""
        echo "Add this to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
        echo ""
        echo "    export PATH=\"\$PATH:${INSTALL_DIR}\""
        echo ""
    fi
}

main() {
    info "Installing ${BINARY_NAME}..."

    local os arch version
    os=$(detect_os)
    arch=$(detect_arch)
    version=$(get_latest_version)

    download_binary "$version" "$os" "$arch"
    check_path

    info "Successfully installed ${BINARY_NAME} ${version}"
    echo ""
    echo "Run '${BINARY_NAME} --help' to get started"
}

main
