# 📥 Installation & Build Guide

This guide covers system prerequisites, automated installation, manual pre-compiled binary deployment, and building Raspiducky from source (including cross-compilation for ARM target hardware).

---

## 📋 System Requirements

To run Raspiducky in USB Gadget mode, your target hardware and operating system must meet the following criteria:

* **Operating System**: Linux with kernel 4.19 or higher (Raspberry Pi OS Lite recommended).
* **Kernel Drivers**: USB Gadget ConfigFS support (`libcomposite` kernel module) and low-level USB controller driver (`dwc2`, `dwc3`, or `sunxi-musb`).
* **Root Privileges**: `root` access is required to interact with `/sys/kernel/config/usb_gadget` and `/dev/hidg*` HID character devices.
* **DebugFS**: `/sys/kernel/debug` mounted for hardware endpoint limit detection.

---

## ⚡ Option 1: Automated One-Line Installation (Recommended)

The automated installation script detects target architecture, downloads the latest binary release, configures kernel overlays, creates udev rules, and installs a systemd background service.

Run this single command on your Raspberry Pi:

```bash
curl -fsSL https://raw.githubusercontent.com/arrase/Raspiducky/master/install.sh | sudo sh
```

### What the Installation Script Performs

1. Detects system architecture (`armv6l`, `armv7l`, `aarch64`, or `x86_64`).
2. Downloads the matching pre-compiled release binary to `/usr/local/bin/raspiducky`.
3. Appends `dtoverlay=dwc2` to `/boot/config.txt` or `/boot/firmware/config.txt`.
4. Ensures `dwc2` and `libcomposite` are loaded in `/etc/modules`.
5. Configures `/etc/fstab` to persist `/sys/kernel/debug` mount.
6. Installs `/etc/udev/rules.d/99-raspiducky-hid.rules` for `/dev/hidg*` device access.
7. Enables and starts the `raspiducky.service` background daemon.

> [!NOTE]
> If installing for the first time on a fresh Raspberry Pi OS image, a **system reboot** is required to activate the kernel `dwc2` overlay:
> ```bash
> sudo reboot
> ```

---

## 📦 Option 2: Pre-Compiled Binary Release Download

If you prefer to install manually without the automated script, download the pre-compiled binary matching your CPU architecture from the GitHub Releases page:

```bash
# 1. Download target binary (Example: Raspberry Pi Zero / ARMv6)
curl -fsSL -o raspiducky https://github.com/arrase/Raspiducky/releases/latest/download/raspiducky-linux-armv6

# 2. Grant executable permissions
chmod +x raspiducky

# 3. Move binary to system path
sudo mv raspiducky /usr/local/bin/raspiducky

# 4. Prepare storage directory
sudo mkdir -p /var/lib/raspiducky

# 5. Enable kernel modules manually
sudo modprobe dwc2
sudo modprobe libcomposite

# 6. Run daemon
sudo /usr/local/bin/raspiducky daemon -port :8000 -storage /var/lib/raspiducky -layout US
```

---

## 🏗️ Option 3: Building from Source

Raspiducky is written in pure Go (ECMAScript engine and USB ConfigFS integration require zero CGO dependencies).

### Prerequisites

* Go 1.22 or higher installed on your build machine (`go version`).
* Git installed (`git`).

### Native Build (On Target Device)

To build directly on your Raspberry Pi:

```bash
# Clone project repository
git clone https://github.com/arrase/Raspiducky.git
cd Raspiducky

# Compile binary
go build -ldflags="-s -w" -o build/raspiducky ./cmd/raspiducky

# Verify binary execution
sudo ./build/raspiducky version
```

---

## 🔀 Cross-Compilation Commands

You can cross-compile Raspiducky from any standard PC (x86_64 / macOS / Linux) for any supported ARM target board without needing external C toolchains (`CGO_ENABLED=0`).

### 1. Raspberry Pi Zero / Zero W (ARMv6 32-bit)
```bash
GOOS=linux GOARCH=arm GOARM=6 CGO_ENABLED=0 go build -ldflags="-s -w" -o build/raspiducky-linux-armv6 ./cmd/raspiducky
```

### 2. Raspberry Pi 3A+ / Banana Pi / Orange Pi (ARMv7 32-bit)
```bash
GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build -ldflags="-s -w" -o build/raspiducky-linux-armv7 ./cmd/raspiducky
```

### 3. Raspberry Pi Zero 2 W / Pi 4 / Pi 5 / Rock Pi (ARM64 64-bit)
```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o build/raspiducky-linux-arm64 ./cmd/raspiducky
```

### 4. Standard PC / Intel NUC (x86_64 64-bit)
```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o build/raspiducky-linux-amd64 ./cmd/raspiducky
```
