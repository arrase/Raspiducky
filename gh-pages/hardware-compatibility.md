# 🖥️ Hardware Compatibility & USB OTG Matrix

Raspiducky turns Single Board Computers (SBCs) into dynamic USB Human Interface Devices (HID), Mass Storage drives, Serial consoles, and Ethernet adapters using Linux kernel ConfigFS (`/sys/kernel/config/usb_gadget`) and USB Device Controller (UDC) drivers.

However, **not all single board computer USB ports hardware-support USB Gadget mode**. This document details hardware compatibility rules, physical endpoint budgets, and release binary selections.

---

## ⚡ USB OTG Requirements & Port Limitations

To operate in USB Gadget mode, an SBC must expose a USB controller capable of operating as a **USB Peripheral/Device (OTG)** rather than strictly as a **USB Host**.

### Why Standard Type-A Ports on RPi 1B / 2B / 3B / 3B+ Are Unsupported

On full-sized Raspberry Pi models 1B, 2B, 3B, and 3B+, standard Type-A ports **cannot be used for USB Gadget mode**:

```text
 Raspberry Pi 3B / 3B+ Architecture (UNSUPPORTED FOR GADGET MODE):

 +-------------------+      USB 2.0 Host       +------------------------+
 | Broadcom SoC      | ----------------------> | SMSC/Microchip LAN9514 |
 | (BCM2837)         |                         | / LAN7515 USB Hub Chip |
 +-------------------+                         +-----------+------------+
                                                           |
                                     +---------------------+---------------------+
                                     |                     |                     |
                                     v                     v                     v
                             [ Type-A Port 1 ]     [ Type-A Port 2 ]     [ Ethernet Port ]
```

* **Onboard USB Hub Chip**: The Broadcom SoC connects its single USB 2.0 channel directly to an onboard multi-port USB Hub / Ethernet combo chip (`LAN9512`, `LAN9514`, or `LAN7515`).
* **Host-Only Topology**: That hub locks the upstream channel permanently into USB Host mode and prevents peripheral enumeration with an external host computer.

---

## 📋 Supported SBC Boards & OTG Port Mapping

| Board Model | Compatible USB Port | Architecture | UDC Controller | Notes |
| :--- | :--- | :--- | :--- | :--- |
| **Raspberry Pi Zero / Zero W** | Micro-USB (`USB` data port) | ARMv6 (32-bit) | `dwc2` (DesignWare USB2) | Compact low-power form factor (`7` endpoints). |
| **Raspberry Pi Zero 2 W** | Micro-USB (`USB` data port) | ARM64 / ARMv7 | `dwc2` (DesignWare USB2) | Quad-core ARM Cortex-A53 (`7` endpoints). |
| **Raspberry Pi Model A / A+** | Standard Type-A OTG Port | ARMv6 (32-bit) | `dwc2` (DesignWare USB2) | Direct SoC USB lane without onboard hub. |
| **Raspberry Pi 3A+** | Standard Type-A OTG Port | ARM64 / ARMv7 | `dwc2` (DesignWare USB2) | Direct SoC USB lane without onboard hub. |
| **Raspberry Pi 4 Model B** | USB-C Power/Data Port | ARM64 (64-bit) | `dwc2` (DesignWare USB2) | Uses `dtoverlay=dwc2,dr_mode=peripheral` (`7` endpoints). |
| **Raspberry Pi 5** | USB-C Power/Data Port | ARM64 (64-bit) | `dwc2` (DesignWare USB2) | Uses `dtoverlay=dwc2,dr_mode=peripheral` (`7` endpoints). |
| **Banana Pi M2 Zero / M1** | Micro-USB OTG Port | ARMv7 (32-bit) | `sunxi-musb` | Compatible with Allwinner H2+/H3/A20 SoCs. |
| **Orange Pi Zero / 1 / 2 / 3 / 5** | Micro-USB / USB-C OTG Port | ARMv7 / ARM64 | `sunxi-musb` / `dwc3` | Check board-specific OTG port silkscreen. |
| **NanoPi NEO / R2S / R4S** | Micro-USB / USB-C OTG Port | ARMv7 / ARM64 | `dwc2` / `dwc3` | Compact headless SBC targets. |
| **Radxa Rock Pi 4 / 5** | USB-C OTG Port | ARM64 (64-bit) | `dwc3` (DesignWare USB3) | Rockchip RK3399 / RK3588 USB 3.0 OTG (`15+` endpoints). |

---

## 🚦 Hardware Endpoint Limits

USB controllers expose a finite number of physical hardware endpoints (`EP0` + `IN` + `OUT`):

```text
             Total Hardware Endpoint Budget Comparison

  DWC2 Controller (RPi Zero / Zero W / Zero 2 W / A+ / 3A+ / 4B / Pi 5):
  [ EP0 ] [ EP1 ] [ EP2 ] [ EP3 ] [ EP4 ] [ EP5 ] [ EP6 ]  ---> MAX 7 Endpoints (num_dev_ep: 7)

  DWC3 Controller (Rock Pi 4 / 5 / Orange Pi 5 / RK3399 / RK3588):
  [ EP0 ] [ EP1 ] [ EP2 ] ... [ EP14 ] [ EP15 ]           ---> MAX 15+ Endpoints
```

### Endpoint Consumption Table

Raspiducky configures `no_out_endpoint=1` on both HID functions (`hid.usb0` and `hid.usb1`), routing keyboard LED `SET_REPORT` updates over `EP0` and saving 2 scarce hardware endpoints on `dwc2` boards:

| Resource / Function | ConfigFS Module | IN Endpoints | OUT Endpoints | Total Cost |
| :--- | :--- | :---: | :---: | :---: |
| 🔹 **Control Endpoint (`EP0`)** | *(Shared)* | — | — | **1** |
| ⌨️ **HID Keyboard** | `hid.usb0` | 1 | 0 (`EP0`) | **1** |
| 🖱️ **HID Mouse** | `hid.usb1` | 1 | 0 | **1** |
| 💾 **Mass Storage (UMS)** | `mass_storage.usb0` | 1 | 1 | **2** |
| 🌐 **USB Ethernet (RNDIS)** | `rndis.usb0` | 2 | 1 | **3** |
| 🌐 **USB Ethernet (CDC ECM)** | `ecm.usb0` | 2 | 1 | **3** |
| 🔌 **USB Serial (CDC ACM)** | `acm.usb0` | 2 | 1 | **3** |

!!! important "Dynamic Endpoint Validation"
    On `dwc2` controllers (`num_dev_ep: 7`), enabling **all** functions simultaneously would require `1 + 1 + 1 + 2 + 3 + 3 + 3 = 14` endpoints. Raspiducky dynamically queries `/sys/kernel/debug/usb/<udc>/hw_params` and prevents deploying profiles whose `usedEndpoints` exceed `maxEndpoints`. Because `RNDIS` and `CDC ECM` are independently selectable, you can comfortably combine `Keyboard (1) + Mouse (1) + ECM or RNDIS (3) + EP0 (1) = 6 / 7` endpoints on any Raspberry Pi OTG model.

---

## 📦 Binary Asset Matrix

Pre-compiled static release binaries (`CGO_ENABLED=0`) are published on GitHub Releases:

| Release Asset Name | Architecture | Target SBC & CPU Models | Recommended OS Image |
| :--- | :--- | :--- | :--- |
| **`raspiducky-linux-armv6`** | ARMv6 32-bit (`GOARM=6`) | • Raspberry Pi Zero / Zero W<br>• Raspberry Pi Model A / A+ | Raspberry Pi OS Lite (32-bit) |
| **`raspiducky-linux-armv7`** | ARMv7 32-bit (`GOARM=7`) | • Raspberry Pi 3A+ (32-bit OS)<br>• Banana Pi M2 Zero / M1<br>• Orange Pi Zero / One / PC<br>• NanoPi NEO | Raspberry Pi OS Lite (32-bit), Armbian |
| **`raspiducky-linux-arm64`** | ARM64 64-bit (`aarch64`) | • Raspberry Pi Zero 2 W (64-bit OS)<br>• Raspberry Pi 3A+ (64-bit OS)<br>• Raspberry Pi 4B / Pi 5<br>• Orange Pi 2 / 3 / 5<br>• NanoPi R2S / R4S<br>• Radxa Rock Pi | Raspberry Pi OS Lite (64-bit), Ubuntu Server, Armbian |
| **`raspiducky-linux-amd64`** | x86_64 64-bit | • Intel NUC / Standard x86_64 PC<br>• Testing with `dummy_hcd` | Debian / Ubuntu / Arch Linux |
