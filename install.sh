#!/bin/sh
# Raspiducky - One-line Installation & Update Script
# Usage: curl -fsSL https://raw.githubusercontent.com/arrase/Raspiducky/master/install.sh | sudo sh

set -eu

REPO="arrase/Raspiducky"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="raspiducky"
SERVICE_NAME="raspiducky.service"
DATA_DIR="/var/lib/raspiducky"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info() {
    printf '%b\n' "${BLUE}[INFO]${NC} $1"
}

success() {
    printf '%b\n' "${GREEN}[SUCCESS]${NC} $1"
}

warn() {
    printf '%b\n' "${YELLOW}[WARN]${NC} $1"
}

error() {
    printf '%b\n' "${RED}[ERROR]${NC} $1" >&2
    exit 1
}

# 1. Check Root Privileges
if [ "$(id -u)" -ne 0 ]; then
    error "This script must be run as root. Please run with sudo or as root user: curl -fsSL ... | sudo sh"
fi

SEPARATOR="====================================================="
printf '%s\n' "${SEPARATOR}"
printf '%s\n' "  Raspiducky USB Gadget & DuckyScript Appliance"
printf '%s\n' "  Installer & Auto-Updater"
printf '%s\n' "${SEPARATOR}"

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

# 4. Resolve Release Binary URL & Download into Secure Temporary Directory
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT INT TERM
TMP_BINARY="${TMP_DIR}/${BINARY_NAME}"

info "Checking latest release version..."
RELEASE_JSON=$($FETCH "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null || true)
LATEST_TAG=""
DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${ASSET_NAME}"

if [ -n "$RELEASE_JSON" ]; then
    LATEST_TAG=$(printf '%s\n' "$RELEASE_JSON" | grep '"tag_name":' | head -n 1 | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -n "$LATEST_TAG" ]; then
        info "Latest release version: ${LATEST_TAG}"
        DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_TAG}/${ASSET_NAME}"
    fi
fi

info "Downloading binary from: ${DOWNLOAD_URL}"

DOWNLOAD_SUCCESS=0
if $FETCH_OUT "$TMP_BINARY" "$DOWNLOAD_URL" 2>/dev/null && [ -s "$TMP_BINARY" ]; then
    DOWNLOAD_SUCCESS=1
fi

if [ "$DOWNLOAD_SUCCESS" -ne 1 ]; then
    warn "Release binary not found at ${DOWNLOAD_URL}"
    info "Searching for local fallback binary..."

    if [ -f "/home/pi/raspiducky" ]; then
        cp /home/pi/raspiducky "$TMP_BINARY"
        success "Using existing binary from /home/pi/raspiducky."
    elif [ -f "./build/${ASSET_NAME}" ]; then
        cp "./build/${ASSET_NAME}" "$TMP_BINARY"
        success "Using local build binary ./build/${ASSET_NAME}."
    elif [ -f "./build/raspiducky" ]; then
        cp ./build/raspiducky "$TMP_BINARY"
        success "Using local build binary ./build/raspiducky."
    else
        error "Failed to download $ASSET_NAME and no local fallback binary found for architecture $UNAME_M."
    fi
fi

# 5. Install Binary
info "Installing binary to ${INSTALL_DIR}/${BINARY_NAME}..."
mkdir -p "$INSTALL_DIR"
install -m 0755 "$TMP_BINARY" "${INSTALL_DIR}/${BINARY_NAME}"
success "Raspiducky binary installed successfully."

# 6. Configure Kernel & USB Gadget Overlay (Raspberry Pi / Linux)
info "Checking USB Gadget & DWC2 kernel overlay configuration..."

CONFIG_TXT=""
if [ -f "/boot/firmware/config.txt" ]; then
    CONFIG_TXT="/boot/firmware/config.txt"
elif [ -f "/boot/config.txt" ]; then
    CONFIG_TXT="/boot/config.txt"
fi

REBOOT_REQUIRED=0

if [ -n "$CONFIG_TXT" ]; then
    if grep -q "^otg_mode=1" "$CONFIG_TXT"; then
        sed -i 's/^otg_mode=1/#otg_mode=1/' "$CONFIG_TXT"
        REBOOT_REQUIRED=1
        info "Disabled 'otg_mode=1' in $CONFIG_TXT for DWC2 gadget compatibility."
    fi

    if grep -q "^dtoverlay=dwc2,dr_mode=host" "$CONFIG_TXT"; then
        sed -i 's/^dtoverlay=dwc2,dr_mode=host/#dtoverlay=dwc2,dr_mode=host/' "$CONFIG_TXT"
        REBOOT_REQUIRED=1
        info "Disabled host-only 'dtoverlay=dwc2,dr_mode=host' in $CONFIG_TXT."
    fi

    if grep -q "^dtoverlay=dwc2$" "$CONFIG_TXT"; then
        sed -i 's/^dtoverlay=dwc2$/dtoverlay=dwc2,dr_mode=peripheral/' "$CONFIG_TXT"
        REBOOT_REQUIRED=1
        success "Updated dwc2 overlay to 'dtoverlay=dwc2,dr_mode=peripheral' in $CONFIG_TXT"
    elif ! grep -q "^dtoverlay=dwc2,dr_mode=peripheral" "$CONFIG_TXT"; then
        info "Adding 'dtoverlay=dwc2,dr_mode=peripheral' under [all] in $CONFIG_TXT..."
        printf '\n[all]\ndtoverlay=dwc2,dr_mode=peripheral\n' >> "$CONFIG_TXT"
        REBOOT_REQUIRED=1
        success "Added 'dtoverlay=dwc2,dr_mode=peripheral' overlay to $CONFIG_TXT"
    else
        info "DWC2 peripheral overlay already present in $CONFIG_TXT."
    fi
fi

# Configure /etc/modules
if [ -f "/etc/modules" ]; then
    if ! grep -q "^dwc2$" /etc/modules; then
        printf '%s\n' "dwc2" >> /etc/modules
        info "Added 'dwc2' to /etc/modules"
    fi
    if ! grep -q "^libcomposite$" /etc/modules; then
        printf '%s\n' "libcomposite" >> /etc/modules
        info "Added 'libcomposite' to /etc/modules"
    fi
fi

# Ensure debugfs is mounted (required for dynamic USB endpoint limits detection)
info "Ensuring debugfs is mounted at /sys/kernel/debug..."
if [ -f "/etc/fstab" ] && ! grep -q "debugfs" /etc/fstab; then
    printf '%s\n' "debugfs /sys/kernel/debug debugfs defaults 0 0" >> /etc/fstab
    info "Added 'debugfs' to /etc/fstab to persist mount across reboots."
fi
if ! mountpoint -q /sys/kernel/debug; then
    mount -t debugfs none /sys/kernel/debug 2>/dev/null || true
    info "Mounted debugfs dynamically."
fi

# Configure udev rules for HID gadget devices (/dev/hidg*)
info "Configuring udev permissions for HID gadget devices (/dev/hidg*)..."
cat << 'EOF' > /etc/udev/rules.d/99-raspiducky-hid.rules
KERNEL=="hidg*", MODE="0660"
EOF
if command -v udevadm >/dev/null 2>&1; then
    udevadm control --reload-rules 2>/dev/null || true
    udevadm trigger --subsystem-match=hidg 2>/dev/null || true
fi
chmod 0660 /dev/hidg* 2>/dev/null || true
success "Configured udev rules and permissions for /dev/hidg*."

# Load kernel modules dynamically if available
info "Loading kernel modules (dwc2, libcomposite)..."
modprobe dwc2 2>/dev/null || true
modprobe libcomposite 2>/dev/null || true

# 7. Create Storage Directory
info "Ensuring persistent storage directory exists at ${DATA_DIR}..."
mkdir -p "$DATA_DIR"

# 8. Create & Enable Systemd Service (no -layout flag so saved gadget_config.json layout is preserved)
if command -v systemctl >/dev/null 2>&1; then
    info "Configuring systemd service (${SERVICE_NAME})..."
    cat << EOF > "/etc/systemd/system/${SERVICE_NAME}"
[Unit]
Description=Raspiducky USB Gadget & DuckyScript Appliance Daemon
After=network.target sys-kernel-config.mount sys-kernel-debug.mount

[Service]
Type=simple
User=root
ExecStart=${INSTALL_DIR}/${BINARY_NAME} daemon -port :8000 -storage ${DATA_DIR}
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
HOST_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
if [ -z "$HOST_IP" ]; then
    HOST_IP="localhost"
fi

printf '\n%s\n' "${SEPARATOR}"
success "Raspiducky installation completed successfully!"
printf '%s\n\n' "${SEPARATOR}"
printf '%s\n' "Installation Path : ${INSTALL_DIR}/${BINARY_NAME}"
printf '%s\n' "Data Directory    : ${DATA_DIR}"
printf '%s\n' "Systemd Service   : ${SERVICE_NAME} (Active)"
printf '\n%s\n\n' "Web Dashboard URL : http://${HOST_IP}:8000"
printf '%s\n' "Commands:"
printf '%s\n' "  - Check Service    : sudo systemctl status ${SERVICE_NAME}"
printf '%s\n' "  - View Logs        : sudo journalctl -u ${SERVICE_NAME} -f"
printf '%s\n\n' "  - Run Payload CLI  : sudo raspiducky run <script_path>"

if [ "$REBOOT_REQUIRED" -eq 1 ]; then
    warn "A new USB overlay (dtoverlay=dwc2,dr_mode=peripheral) was added to your kernel config."
    warn "If this is the first installation, please REBOOT your Raspberry Pi to activate USB Gadget hardware mode:"
    warn "  sudo reboot"
fi
