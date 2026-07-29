#!/bin/sh
# Raspiducky - One-line Installation & Update Script
# Usage: curl -fsSL https://raw.githubusercontent.com/arrase/Raspiducky/master/install.sh | sudo sh

set -e

REPO="arrase/Raspiducky"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="raspiducky"
SERVICE_NAME="raspiducky.service"
DATA_DIR="/var/lib/raspiducky"

# Colors for terminal output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

info() {
    echo "${BLUE}[INFO]${NC} $1"
}

success() {
    echo "${GREEN}[SUCCESS]${NC} $1"
}

warn() {
    echo "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo "${RED}[ERROR]${NC} $1" >&2
    exit 1
}

# 1. Check Root Privileges
if [ "$(id -u)" -ne 0 ]; then
    error "This script must be run as root. Please run with sudo or as root user: curl -fsSL ... | sudo sh"
fi

echo "====================================================="
echo "  🦆 Raspiducky USB Gadget & DuckyScript Appliance   "
echo "  Installer & Auto-Updater                           "
echo "====================================================="

# 2. Detect System Architecture
UNAME_M=$(uname -m)
case "$UNAME_M" in
    armv6l)
        ASSET_NAME="raspiducky-linux-armv6"
        ;;
    armv7l)
        ASSET_NAME="raspiducky-linux-armv7"
        ;;
    aarch64|arm64)
        ASSET_NAME="raspiducky-linux-arm64"
        ;;
    x86_64|amd64)
        ASSET_NAME="raspiducky-linux-amd64"
        ;;
    *)
        error "Unsupported system architecture: $UNAME_M"
        ;;
esac

info "Detected architecture: $UNAME_M (Asset: $ASSET_NAME)"

# 3. Detect Download Tool (curl or wget)
if command -v curl >/dev/null 2>&1; then
    FETCH="curl -fsSL"
    FETCH_OUT="curl -fsSL -o"
elif command -v wget >/dev/null 2>&1; then
    FETCH="wget -qO-"
    FETCH_OUT="wget -q -O"
else
    error "Neither 'curl' nor 'wget' was found. Please install curl or wget."
fi

# 4. Fetch Latest Version & Binary Download URL
info "Checking latest release version from GitHub API..."

RELEASE_JSON=""
if [ -n "$FETCH" ]; then
    RELEASE_JSON=$($FETCH "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null || true)
fi

DOWNLOAD_URL=""
LATEST_TAG=""

if [ -n "$RELEASE_JSON" ]; then
    LATEST_TAG=$(echo "$RELEASE_JSON" | grep '"tag_name":' | head -n 1 | sed -E 's/.*"([^"]+)".*/\1/')
    # Search for matching asset browser_download_url
    DOWNLOAD_URL=$(echo "$RELEASE_JSON" | grep "browser_download_url" | grep -i "$ASSET_NAME" | head -n 1 | sed -E 's/.*"([^"]+)".*/\1/')
    
    # Fallback asset name matching if specific arch asset isn't found
    if [ -z "$DOWNLOAD_URL" ]; then
        DOWNLOAD_URL=$(echo "$RELEASE_JSON" | grep "browser_download_url" | grep -i "raspiducky" | head -n 1 | sed -E 's/.*"([^"]+)".*/\1/')
    fi
fi

if [ -n "$LATEST_TAG" ]; then
    info "Latest release version: ${LATEST_TAG}"
fi

if [ -z "$DOWNLOAD_URL" ]; then
    if [ -n "$LATEST_TAG" ]; then
        DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_TAG}/${ASSET_NAME}"
    else
        DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${ASSET_NAME}"
    fi
fi

TMP_BINARY="/tmp/${BINARY_NAME}_tmp"
rm -f "$TMP_BINARY"

info "Downloading binary from: ${DOWNLOAD_URL}"

DOWNLOAD_SUCCESS=0
if $FETCH_OUT "$TMP_BINARY" "$DOWNLOAD_URL" 2>/dev/null && [ -s "$TMP_BINARY" ]; then
    DOWNLOAD_SUCCESS=1
fi

if [ "$DOWNLOAD_SUCCESS" -ne 1 ]; then
    warn "Release binary not found at ${DOWNLOAD_URL}"
    info "Searching for existing binary at /home/pi/raspiducky, ./build/raspiducky-armv6 or ./build/raspiducky..."
    
    if [ -f "/home/pi/raspiducky" ]; then
        cp /home/pi/raspiducky "$TMP_BINARY"
        success "Using existing binary from /home/pi/raspiducky."
    elif [ -f "./build/raspiducky-armv6" ]; then
        cp ./build/raspiducky-armv6 "$TMP_BINARY"
        success "Using local build binary ./build/raspiducky-armv6."
    elif [ -f "./build/raspiducky" ]; then
        cp ./build/raspiducky "$TMP_BINARY"
        success "Using local build binary ./build/raspiducky."
    else
        error "Failed to download $ASSET_NAME and no local fallback binary found. Once a release with binaries is published on GitHub, the installer will download it automatically."
    fi
fi

chmod +x "$TMP_BINARY"

# 5. Install Binary
info "Installing binary to ${INSTALL_DIR}/${BINARY_NAME}..."
mkdir -p "$INSTALL_DIR"
mv "$TMP_BINARY" "${INSTALL_DIR}/${BINARY_NAME}"
success "Raspiducky binary installed successfully."

# 6. Configure Kernel & USB Gadget Overlay (Raspberry Pi / Linux)
info "Checking USB Gadget & DWC2 kernel overlay configuration..."

# Find config.txt
CONFIG_TXT=""
if [ -f "/boot/firmware/config.txt" ]; then
    CONFIG_TXT="/boot/firmware/config.txt"
elif [ -f "/boot/config.txt" ]; then
    CONFIG_TXT="/boot/config.txt"
fi

REBOOT_REQUIRED=0

if [ -n "$CONFIG_TXT" ]; then
    if ! grep -q "^dtoverlay=dwc2" "$CONFIG_TXT"; then
        info "Adding 'dtoverlay=dwc2' to $CONFIG_TXT..."
        echo "dtoverlay=dwc2" >> "$CONFIG_TXT"
        REBOOT_REQUIRED=1
        success "Added dwc2 overlay to $CONFIG_TXT"
    else
        info "DWC2 overlay already present in $CONFIG_TXT."
    fi
fi

# Configure /etc/modules
if [ -f "/etc/modules" ]; then
    if ! grep -q "^dwc2$" /etc/modules; then
        echo "dwc2" >> /etc/modules
        info "Added 'dwc2' to /etc/modules"
    fi
    if ! grep -q "^libcomposite$" /etc/modules; then
        echo "libcomposite" >> /etc/modules
        info "Added 'libcomposite' to /etc/modules"
    fi
fi

# Ensure debugfs is mounted (required for dynamic USB endpoint limits detection)
info "Ensuring debugfs is mounted at /sys/kernel/debug..."
if [ -f "/etc/fstab" ]; then
    if ! grep -q "debugfs" /etc/fstab; then
        echo "debugfs /sys/kernel/debug debugfs defaults 0 0" >> /etc/fstab
        info "Added 'debugfs' to /etc/fstab to persist mount across reboots."
    fi
fi
if ! mountpoint -q /sys/kernel/debug; then
    mount -t debugfs none /sys/kernel/debug 2>/dev/null || true
    info "Mounted debugfs dynamically."
fi

# Configure udev rules for HID gadget devices (/dev/hidg*)
info "Configuring udev permissions for HID gadget devices (/dev/hidg*)..."
cat << 'EOF' > /etc/udev/rules.d/99-raspiducky-hid.rules
KERNEL=="hidg*", MODE="0666"
EOF
if command -v udevadm >/dev/null 2>&1; then
    udevadm control --reload-rules 2>/dev/null || true
    udevadm trigger --subsystem-match=hidg 2>/dev/null || true
fi
chmod 666 /dev/hidg* 2>/dev/null || true
success "Configured udev rules and permissions for /dev/hidg*."

# Load kernel modules dynamically if available
info "Loading kernel modules (dwc2, libcomposite)..."
modprobe dwc2 2>/dev/null || true
modprobe libcomposite 2>/dev/null || true

# 7. Create Storage Directory
info "Ensuring persistent storage directory exists at ${DATA_DIR}..."
mkdir -p "$DATA_DIR"

# 8. Create & Enable Systemd Service
if command -v systemctl >/dev/null 2>&1; then
    info "Configuring systemd service (${SERVICE_NAME})..."
    cat << EOF > "/etc/systemd/system/${SERVICE_NAME}"
[Unit]
Description=Raspiducky USB Gadget & DuckyScript Appliance Daemon
After=network.target

[Service]
Type=simple
User=root
ExecStart=${INSTALL_DIR}/${BINARY_NAME} daemon -port :8000 -storage ${DATA_DIR} -layout US
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable "${SERVICE_NAME}"
    systemctl restart "${SERVICE_NAME}"
    success "Systemd service ${SERVICE_NAME} enabled and started."
fi

# 9. Installation Complete Summary
echo ""
echo "====================================================="
success "Raspiducky installation completed successfully! 🦆⚡"
echo "====================================================="
echo ""
echo "📍 Installation Path : ${INSTALL_DIR}/${BINARY_NAME}"
echo "📁 Data Directory    : ${DATA_DIR}"
echo "⚙️ Systemd Service   : ${SERVICE_NAME} (Active)"
echo ""
echo "🌐 Web Dashboard URL : http://$(hostname -I 2>/dev/null | awk '{print $1}' || echo "localhost"):8000"
echo ""
echo "💡 Commands:"
echo "  - Check Service    : sudo systemctl status ${SERVICE_NAME}"
echo "  - View Logs        : sudo journalctl -u ${SERVICE_NAME} -f"
echo "  - Run Payload CLI  : sudo raspiducky run <script_path>"
echo ""

if [ "$REBOOT_REQUIRED" -eq 1 ]; then
    warn "A new USB overlay (dtoverlay=dwc2) was added to your kernel config."
    warn "If this is the first installation, please REBOOT your Raspberry Pi to activate USB Gadget hardware mode:"
    warn "  sudo reboot"
fi

echo ""
