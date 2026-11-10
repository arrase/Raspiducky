# 📥 Installation & Build Guide

This guide covers system prerequisites, one-line automated installation, manual binary deployment, and building or cross-compiling Raspiducky from source.

---

## 📋 System Requirements

* **Operating System**: Linux with kernel 4.19+ (Raspberry Pi OS Lite recommended).
* **Kernel Modules**: USB Gadget ConfigFS (`libcomposite`) and a peripheral-capable UDC driver (`dwc2`, `dwc3`, or `sunxi-musb`).
* **Root Privileges**: `root` access is required to configure `/sys/kernel/config/usb_gadget` and open `/dev/hidg*` character devices.
* **DebugFS**: `/sys/kernel/debug` mounted so Raspiducky can query hardware endpoint limits (`hw_params`).

---

## ⚡ Option 1: Automated One-Line Installation (Recommended)

Run the official installer on your Raspberry Pi or compatible Linux SBC:

```bash
curl -fsSL https://raw.githubusercontent.com/arrase/Raspiducky/master/install.sh | sudo sh
```

### What `install.sh` Performs

1. Detects CPU architecture (`armv6l`, `armv7l`, `aarch64`, or `x86_64`).
2. Downloads the latest release binary into an isolated `mktemp -d` staging directory and installs it to `/usr/local/bin/raspiducky`.
3. Disables conflicting host-only boot settings (`otg_mode=1` and `dtoverlay=dwc2,dr_mode=host`) and adds `dtoverlay=dwc2,dr_mode=peripheral` under `[all]` in `/boot/firmware/config.txt` (or `/boot/config.txt`).
4. Registers `dwc2` and `libcomposite` in `/etc/modules` and loads them via `modprobe`.
5. Persists `debugfs` (`/sys/kernel/debug`) in `/etc/fstab` and mounts it immediately.
6. Installs `/etc/udev/rules.d/99-raspiducky-hid.rules` (`MODE="0660"`) for `/dev/hidg*`.
7. Creates `/var/lib/raspiducky` and enables `raspiducky.service` (ordered after `sys-kernel-config.mount` and `sys-kernel-debug.mount`, preserving saved layout and gadget profiles across updates).

!!! note "First-Time Reboot"
    When installing on a fresh Raspberry Pi OS image where `dtoverlay=dwc2,dr_mode=peripheral` was newly added to `config.txt`, reboot once to activate the USB peripheral controller:
    ```bash
    sudo reboot
    ```

---

## 📦 Option 2: Manual Pre-Compiled Binary Deployment

Download the binary matching your board's architecture from [GitHub Releases](https://github.com/arrase/Raspiducky/releases/latest):

```bash
# 1. Download target binary (Example: Raspberry Pi 4 / Zero 2 W 64-bit)
curl -fsSL -o raspiducky https://github.com/arrase/Raspiducky/releases/latest/download/raspiducky-linux-arm64

# 2. Install executable
chmod +x raspiducky
sudo mv raspiducky /usr/local/bin/raspiducky

# 3. Create persistent storage directory & load kernel modules
sudo mkdir -p /var/lib/raspiducky
sudo modprobe dwc2
sudo modprobe libcomposite

# 4. Run daemon
sudo /usr/local/bin/raspiducky daemon -port :8000 -storage /var/lib/raspiducky
```

---

## 🏗️ Option 3: Building from Source

Raspiducky is written in pure Go (`CGO_ENABLED=0`) with zero external C dependencies.

### Prerequisites

* **Go 1.26+** (`go version`)
* **Git** (`git`)

### Native Build

```bash
git clone https://github.com/arrase/Raspiducky.git
cd Raspiducky

go build -ldflags="-s -w -X main.version=dev" -o build/raspiducky ./cmd/raspiducky
./build/raspiducky version
```

---

## 🔀 Cross-Compilation Commands

Cross-compile static Linux binaries from any workstation (Linux, macOS, or Windows) without C cross-toolchains:

### 1. Raspberry Pi Zero / Zero W / A+ (ARMv6 32-bit)
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build -ldflags="-s -w -X main.version=v2.5.0" -o build/raspiducky-linux-armv6 ./cmd/raspiducky
```

### 2. Raspberry Pi 3A+ / Banana Pi / Orange Pi (ARMv7 32-bit)
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -ldflags="-s -w -X main.version=v2.5.0" -o build/raspiducky-linux-armv7 ./cmd/raspiducky
```

### 3. Raspberry Pi Zero 2 W / Pi 4 / Pi 5 / Rock Pi (ARM64 64-bit)
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w -X main.version=v2.5.0" -o build/raspiducky-linux-arm64 ./cmd/raspiducky
```

### 4. Standard PC / Intel NUC (x86_64 64-bit)
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.version=v2.5.0" -o build/raspiducky-linux-amd64 ./cmd/raspiducky
```
