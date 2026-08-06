# USB Gadget Functions & Hardware Limits

Raspiducky turns Linux Single Board Computers into multi-function USB composite devices. Through Linux ConfigFS, Raspiducky can dynamically initialize and combine up to 5 distinct USB gadget functions.

---

## 🔌 Technical Breakdown of USB Functions

### 1. ⌨️ HID Keyboard (`hid.usb0`)

The HID Keyboard function exposes a standard USB boot-protocol keyboard device (`/dev/hidg0`).

* **Report Format (8 Bytes)**:
  - Byte 0: Modifier keys bitmask (`Ctrl`, `Shift`, `Alt`, `GUI`)
  - Byte 1: Reserved (0x00)
  - Bytes 2-7: Array of up to 6 simultaneous USB keycodes (NKRO limitation to 6-key rollover)

```text
+----------+----------+----------+----------+----------+----------+----------+----------+
| Modifiers| Reserved | KeyCode1 | KeyCode2 | KeyCode3 | KeyCode4 | KeyCode5 | KeyCode6 |
|  (Byte 0)| (Byte 1) | (Byte 2) | (Byte 3) | (Byte 4) | (Byte 5) | (Byte 6) | (Byte 7) |
+----------+----------+----------+----------+----------+----------+----------+----------+
```

* **Multi-Layout Translation Tables**:
  Physical keystrokes vary across regional keyboard layouts. Raspiducky includes built-in character mapping tables for:
  - `US`: Standard English (US)
  - `ES`: Spanish (Spain / Latin America)
  - `DE`: German (QWERTZ layout)
  - `FR`: French (AZERTY layout)

---

### 2. 🖱️ HID Mouse (`hid.usb1`)

The HID Mouse function provides mouse input emulation (`/dev/hidg1`) supporting both relative movement and absolute screen coordinates.

* **Relative Movement Mode (4 Bytes)**:
  - Byte 0: Button mask (`Bit 0: Left`, `Bit 1: Right`, `Bit 2: Middle`)
  - Byte 1: X displacement (`int8`: -127 to +127)
  - Byte 2: Y displacement (`int8`: -127 to +127)
  - Byte 3: Vertical Scroll Wheel (`int8`: -127 to +127)

* **Absolute Position Mode (6 Bytes)**:
  - Allows targeting specific screen coordinates (0 to 32767 normalized range) for precise UI interaction.

---

### 3. 💾 USB Mass Storage - UMS (`mass_storage.usb0`)

The Mass Storage function allows Raspiducky to emulate a USB flash drive or optical disc drive by binding a disk image file (ISO or RAW format) stored on the Pi's filesystem.

* **Emulation Modes**:
  - **Flash Drive Mode**: Writable or read-only disk image (`.raw`, `.img`). Host OS treats the Pi as a standard removable USB flash drive.
  - **CD-ROM Mode**: Emulates an ISO 9660 virtual CD-ROM drive (`.iso`). Useful for automated software installation or bypass configurations where USB drives are restricted but optical media is allowed.
* **ConfigFS Parameters**:
  Configured via `/sys/kernel/config/usb_gadget/raspiducky/functions/mass_storage.usb0/lun.0/`:
  - `file`: Path to disk image (e.g. `/var/lib/raspiducky/storage.img`)
  - `ro`: Read-only flag (`1` for ISO/CD-ROM, `0` for writable disk)
  - `cdrom`: CD-ROM emulation flag (`1` or `0`)
  - `removable`: Set to `1` to support host ejection events.

---

### 4. 🌐 USB Network (`ecm.usb0` / `rndis.usb0`)

Provides virtual network interface adapters over USB, turning the Pi into an Ethernet gateway or network device for the host system.

* **CDC ECM (Ethernet Control Model)**: Standard USB networking protocol natively supported by Linux, macOS, and Android.
* **RNDIS (Remote NDIS)**: Microsoft proprietary protocol required for plug-and-play network driver binding on Windows hosts.
* **Dual Descriptors & OS Strings**: Raspiducky configures Microsoft OS 1.0 descriptors (`qw_sign`, `b_vendor_code`, `compat_id = RNDIS`) so Windows automatically binds the `rndis.sys` driver without requiring manual user driver installation.
* **Custom Addresses**: MAC address, host MAC address, and IP subnets can be defined per deployment.

---

### 5. 🔌 USB Serial Console (`acm.usb0`)

Exposes a USB CDC ACM serial communications interface (`/dev/ttyGS0`).

* **Features**:
  - Enables direct terminal shell access to the Pi over USB.
  - Provides a serial communication interface for automated host-to-Pi data exchange without network setup.

---

## 🚦 Hardware IN Endpoint Limits

USB Device Controllers (UDC) in Single Board Computers have strict hardware limits on the maximum number of **IN Endpoints** available across all active composite gadget functions.

> ⚠️ **Hardware Constraint**: If a composite USB gadget deployment attempts to allocate more IN Endpoints than the physical USB controller supports, the kernel driver will fail to bind to the UDC (`Device or resource busy` or `No space left on device`).

### USB Controller Limit Comparison

| USB Controller | Max IN Endpoints | Example SBC Boards |
| :--- | :---: | :--- |
| **Broadcom DWC2** | **7 IN Endpoints** | • Raspberry Pi Zero / Zero W / Zero 2 W<br>• Raspberry Pi Model A / A+ / 3A+<br>• Raspberry Pi Compute Module 1 / 3 |
| **Synopsys DWC3** | **15 IN Endpoints** | • Raspberry Pi 4B / 5<br>• Raspberry Pi Compute Module 4 / 5<br>• Rockchip RK3399 / RK3588 (Orange Pi 5, Rock Pi) |

---

### Endpoint Consumption Matrix

Each gadget function consumes a specific number of IN endpoints:

| USB Gadget Function | IN Endpoints Consumed | Description |
| :--- | :---: | :--- |
| ⌨️ **HID Keyboard** | **1** | Interrupt IN Endpoint |
| 🖱️ **HID Mouse** | **1** | Interrupt IN Endpoint |
| 💾 **Mass Storage (UMS)** | **1** | Bulk IN Endpoint |
| 🔌 **USB Serial (ACM)** | **2** | Interrupt IN + Bulk IN Endpoints |
| 🌐 **USB Network (ECM + RNDIS)** | **4** | 2 for ECM (Interrupt + Bulk IN) + 2 for RNDIS |

---

### 📋 Example Deployment Combinations (DWC2 Limit: 7)

- ✅ **Combination 1 (Keyboard + Mouse + Mass Storage)**:
  `1 (Kbd) + 1 (Mouse) + 1 (UMS) = 3 IN Endpoints` *(Supported on all Pi boards)*

- ✅ **Combination 2 (Keyboard + Mass Storage + Serial)**:
  `1 (Kbd) + 1 (UMS) + 2 (Serial) = 4 IN Endpoints` *(Supported on all Pi boards)*

- ✅ **Combination 3 (Keyboard + Mouse + Network)**:
  `1 (Kbd) + 1 (Mouse) + 4 (Network) = 6 IN Endpoints` *(Supported on all Pi boards)*

- ❌ **Invalid Combination (Keyboard + Mouse + Serial + Network)**:
  `1 (Kbd) + 1 (Mouse) + 2 (Serial) + 4 (Network) = 8 IN Endpoints` *(Exceeds DWC2 limit of 7; only supported on DWC3 / Pi 4 & 5)*

> 💡 **Automated Protection**: Raspiducky reads hardware capabilities from `/sys/kernel/debug/usb/` and validates endpoint counts before applying ConfigFS profiles, preventing invalid gadget deployments.
