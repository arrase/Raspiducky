# 🖥️ Hardware Compatibility & USB OTG Matrix

Raspiducky turns Single Board Computers (SBCs) into dynamic USB Human Interface Devices (HID), Mass Storage drives, Serial consoles, and Ethernet adapters using Linux kernel ConfigFS (`/sys/kernel/config/usb_gadget`) and USB Device Controller (UDC) drivers.

However, **not all single board computer USB ports hardware-support USB Gadget mode**. This document details hardware compatibility rules, endpoint constraints, and release binary selections.

---

## ⚡ USB OTG Requirements & Port Limitations

To operate in USB Gadget mode, an SBC must expose a USB controller capable of operating as a **USB Peripheral/Device (OTG)** rather than strictly as a **USB Host**.

### Why Standard Type-A Ports on RPi 1B / 2B / 3B / 3B+ Are Unsupported

A common misconception is that any USB port can emulate a USB keyboard or flash drive via software. On full-sized Raspberry Pi models (1B, 2B, 3B, and 3B+), standard Type-A ports **cannot be used for USB Gadget mode**.

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

* **Onboard USB Hub Chip**: The Broadcom SoC connects its single USB 2.0 OTG channel directly to an onboard multi-port USB Hub / Ethernet combo chip (SMSC LAN9512, LAN9514, or LAN7515).
* **Host-Only Topology**: The presence of this active USB hub hardware locks the controller permanently into USB Host mode. It physically prevents the SoC from negotiating downstream USB peripheral descriptors with a connected host computer.

---

## 📋 Supported SBC Boards & OTG Port Mapping

| Board Model | Compatible USB Port | Architecture | UDC Controller | Notes |
| :--- | :--- | :--- | :--- | :--- |
| **Raspberry Pi Zero / Zero W** | Micro-USB (Data Port) | ARMv6 (32-bit) | `dwc2` (DesignWare USB2) | Ideal low-power compact form factor. |
| **Raspberry Pi Zero 2 W** | Micro-USB (Data Port) | ARM64 / ARMv7 | `dwc2` (DesignWare USB2) | High-performance quad-core ARM 64-bit CPU. |
| **Raspberry Pi Model A / A+** | Standard Type-A OTG Port | ARMv6 (32-bit) | `dwc2` (DesignWare USB2) | Direct SoC USB connection (no onboard hub). |
| **Raspberry Pi 3A+** | Standard Type-A OTG Port | ARM64 / ARMv7 | `dwc2` (DesignWare USB2) | Direct SoC USB connection (no onboard hub). |
| **Raspberry Pi 4 Model B** | USB-C Power/Data Port | ARM64 (64-bit) | `dwc3` (DesignWare USB3) | USB-C port supports high-speed OTG. |
| **Raspberry Pi 5** | USB-C Power/Data Port | ARM64 (64-bit) | `dwc3` (DesignWare USB3) | Dual-role USB-C controller support. |
| **Banana Pi M2 / M1** | Micro-USB OTG Port | ARMv7 (32-bit) | `sunxi-musb` | Compatible with Allwinner H3/A20 SoCs. |
| **Orange Pi Zero / 1 / 2 / 3 / 5** | Micro-USB / USB-C OTG Port | ARMv7 / ARM64 | `sunxi-musb` / `dwc3` | Check specific board OTG port labeling. |
| **NanoPi NEO / R2S / R4S** | Micro-USB / USB-C OTG Port | ARMv7 / ARM64 | `dwc2` / `dwc3` | High-density compact hardware targets. |
| **Radxa Rock Pi 4 / 5** | USB-C OTG Port | ARM64 (64-bit) | `dwc3` (DesignWare USB3) | Rockchip RK3399/RK3588 USB 3.0 OTG. |

---

## 🚦 Hardware Endpoint Limits

Single Board Computer USB controllers have physical limitations on how many IN/OUT hardware endpoints can be active at the same time.

```text
             IN Hardware Endpoint Limit Comparison
  
  DWC2 Controller (RPi Zero / Zero W / Zero 2 W / 3A+):
  [ EP0 ] [ EP1 ] [ EP2 ] [ EP3 ] [ EP4 ] [ EP5 ] [ EP6 ]  ---> MAX 7 IN Endpoints
  
  DWC3 Controller (RPi 4B / Pi 5 / RK3399):
  [ EP0 ] [ EP1 ] [ EP2 ] ... [ EP14 ] [ EP15 ]           ---> MAX 15 IN Endpoints
```

### Endpoint Consumption Table

| USB Function | ConfigFS Module Name | IN Endpoints Consumed | OUT Endpoints Consumed |
| :--- | :--- | :---: | :---: |
| ⌨️ **HID Keyboard** | `hid.usb0` | **1** | 0 |
| 🖱️ **HID Mouse** | `hid.usb1` | **1** | 0 |
| 💾 **Mass Storage (UMS)** | `mass_storage.usb0` | **1** | 1 |
| 🔌 **USB Serial (CDC ACM)** | `acm.usb0` | **2** | 1 |
| 🌐 **USB Ethernet (RNDIS + ECM)** | `rndis.usb0` + `ecm.usb0` | **4** (2 per driver) | 2 |

> [!IMPORTANT]
> On `dwc2` controllers (such as Raspberry Pi Zero), deploying **all USB functions simultaneously** requires **9 IN Endpoints**, exceeding the hardware maximum of 7.
>
> Raspiducky dynamically queries `/sys/kernel/debug/usb/<udc>/hw_params` on startup and rejects configurations that exceed endpoint bounds.

---

## 📦 Binary Asset Matrix

Pre-compiled production release binaries are available for download on GitHub Releases. Match your target device hardware to the appropriate binary artifact:

| Release Asset Name | Architecture | Target SBC & CPU Models | Recommended OS Image |
| :--- | :--- | :--- | :--- |
| **`raspiducky-linux-armv6`** | ARMv6 32-bit (`GOARM=6`) | • Raspberry Pi Zero<br>• Raspberry Pi Zero W<br>• Raspberry Pi Model A / A+ | Raspberry Pi OS Lite (32-bit) |
| **`raspiducky-linux-armv7`** | ARMv7 32-bit (`GOARM=7`) | • Raspberry Pi 3A+ (32-bit OS)<br>• Banana Pi M2 Zero / M1<br>• Orange Pi Zero / One / PC<br>• NanoPi NEO | Raspberry Pi OS Lite (32-bit), Armbian |
| **`raspiducky-linux-arm64`** | ARM64 64-bit (`aarch64`) | • Raspberry Pi Zero 2 W (64-bit OS)<br>• Raspberry Pi 3A+ (64-bit OS)<br>• Raspberry Pi 4B / Pi 5<br>• Orange Pi 2 / 3 / 5<br>• NanoPi R2S / R4S<br>• Radxa Rock Pi | Raspberry Pi OS Lite (64-bit), Ubuntu Server, Armbian |
| **`raspiducky-linux-amd64`** | x86_64 64-bit | • Intel NUC / Standard x86 PC<br>• Testing in virtualized environments | Debian / Ubuntu / Arch Linux |
