# USB Gadget Functions & Hardware Limits

Raspiducky turns Linux Single Board Computers into multi-function USB composite devices. Through Linux ConfigFS, Raspiducky dynamically configures and combines up to **6 independent USB gadget functions** without rebooting.

---

## 🔌 Technical Breakdown of USB Functions

### 1. ⌨️ HID Keyboard (`hid.usb0`)

The HID Keyboard function exposes a standard USB boot-protocol keyboard device (`/dev/hidg0`).

* **Endpoint Optimization (`no_out_endpoint=1`)**:
  Configured with `no_out_endpoint=1` so keyboard LED updates (`NumLock`, `CapsLock`, `ScrollLock`) travel over `EP0` (`SET_REPORT` control transfers) rather than consuming a scarce dedicated hardware OUT endpoint.
* **Report Format (8 Bytes)**:
  - Byte 0: Modifier keys bitmask (`Ctrl`, `Shift`, `Alt`, `GUI`, `AltGr`)
  - Byte 1: Reserved (`0x00`)
  - Bytes 2–7: Up to 6 simultaneous USB HID keycodes (6-key rollover)

```text
+----------+----------+----------+----------+----------+----------+----------+----------+
| Modifiers| Reserved | KeyCode1 | KeyCode2 | KeyCode3 | KeyCode4 | KeyCode5 | KeyCode6 |
|  (Byte 0)| (Byte 1) | (Byte 2) | (Byte 3) | (Byte 4) | (Byte 5) | (Byte 6) | (Byte 7) |
+----------+----------+----------+----------+----------+----------+----------+----------+
```

* **Multi-Layout & Dead-Key Translation Tables**:
  Built-in rune and combo mapping tables (including dead-key + `Space` emission for characters like `^`, `` ` ``, `´`, `~` on European layouts and smart typography normalization):
  - `US`: English (US QWERTY)
  - `ES`: Spanish (ISO QWERTY)
  - `DE`: German (QWERTZ)
  - `FR`: French (AZERTY)

---

### 2. 🖱️ HID Mouse (`hid.usb1`)

The HID Mouse function provides composite mouse emulation (`/dev/hidg1`) using a dual-collection **6-byte report descriptor** (`no_out_endpoint=1`) supporting both relative displacement and absolute screen positioning:

* **Relative Movement Report (`Report ID 0x01`, 6 Bytes)**:
  - Byte 0: Report ID (`0x01`)
  - Byte 1: Button bitmask (`0x01`: Left, `0x02`: Right, `0x04`: Middle)
  - Byte 2: X displacement (`int8`: `-127` to `+127`)
  - Byte 3: Y displacement (`int8`: `-127` to `+127`)
  - Bytes 4–5: Padding (`0x00, 0x00`)
  - *Note: Displacements larger than `±127` passed to `mouseMove` / `MOUSE_MOVE` are automatically split across sequential reports.*

* **Absolute Position Report (`Report ID 0x02`, 6 Bytes)**:
  - Byte 0: Report ID (`0x02`)
  - Byte 1: Button bitmask
  - Bytes 2–3: Little-endian `uint16` X coordinate (`0` to `32767`)
  - Bytes 4–5: Little-endian `uint16` Y coordinate (`0` to `32767`)

---

### 3. 💾 USB Mass Storage — UMS (`mass_storage.usb0`)

Emulates a removable USB flash drive or optical CD-ROM drive backed by an image file on the SBC's storage directory (`/var/lib/raspiducky/storage.img`).

* **Emulation Modes**:
  - **Flash Drive Mode (`storageCdrom: false`)**: Writable or read-only raw disk image. Automatically allocates a sparse image file sized via `storageSizeMb` if none exists yet.
  - **CD-ROM Mode (`storageCdrom: true`)**: Emulates an optical ISO 9660 CD-ROM drive (`cdrom=1`, forced read-only `ro=1`).
* **ConfigFS Attributes (`functions/mass_storage.usb0/lun.0/`)**:
  - `file`: Backing image path
  - `ro`: Read-only toggle (`1` or `0`)
  - `cdrom`: Optical drive toggle (`1` or `0`)
  - `removable`: Set to `1` for clean host ejection

---

### 4. 🌐 USB Network (`rndis.usb0` & `ecm.usb0`)

Raspiducky exposes **RNDIS** and **CDC ECM** as **independently toggleable** USB Ethernet functions so you only allocate endpoints for the target host OS you need:

* **RNDIS (`rndis.usb0`)**: Microsoft Remote NDIS protocol for plug-and-play networking on **Windows** hosts. Raspiducky automatically populates Microsoft OS 1.0 descriptors (`qw_sign = MSFT100`, `b_vendor_code = 0xbc`, `compat_id = RNDIS`, `sub_compat_id = 5162001`) so Windows binds `rndis.sys` without manual driver prompts.
* **CDC ECM (`ecm.usb0`)**: Standard USB Ethernet Control Model natively supported on **Linux, macOS, and Android** hosts.
* **Custom MAC Addresses**: Both `host_addr` and `dev_addr` MAC addresses can be customized per interface.

---

### 5. 🔌 USB Serial Console (`acm.usb0`)

Exposes a standard USB CDC ACM virtual serial port (`/dev/ttyGS0`) for direct out-of-band shell access or host-to-SBC serial data channels without network configuration.

---

## 🚦 Hardware Endpoint Limits

USB Device Controllers (UDC) on Single Board Computers have a fixed hardware pool of **total endpoints** (`EP0` + `IN` + `OUT`). On Synopsys DesignWare USB2 (`dwc2`) controllers used across Raspberry Pi boards, `/sys/kernel/debug/usb/<udc>/hw_params` reports `num_dev_ep: 7` (**1 Control `EP0` + 6 function endpoints**).

!!! warning "Hardware Endpoint Budget"
    Attempting to bind a composite gadget that requires more endpoints than the UDC physically provides causes the kernel bind write to `/sys/kernel/config/usb_gadget/raspiducky/UDC` to fail (`ENODEV` / `ENOSPC`). Raspiducky computes `usedEndpoints` against `maxEndpoints` before touching ConfigFS and displays a live endpoint budget bar in the Web Dashboard.

### USB Controller Comparison

| USB Controller | Total Hardware Endpoints | Example SBC Boards |
| :--- | :---: | :--- |
| **Synopsys DWC2 (`dwc2`)** | **7 Endpoints** (`EP0` + 6) | • Raspberry Pi Zero / Zero W / Zero 2 W<br>• Raspberry Pi Model A / A+ / 3A+<br>• Raspberry Pi 4B / 5 *(USB-C `dr_mode=peripheral`)* |
| **Synopsys DWC3 (`dwc3`)** | **15+ Endpoints** | • Radxa Rock Pi 4 / 5 (RK3399 / RK3588)<br>• Orange Pi 5 (RK3588) |

---

### Endpoint Cost Matrix

Because Raspiducky enables `no_out_endpoint=1` on both HID functions, Keyboard and Mouse consume **only 1 endpoint each**:

| Resource / Function | ConfigFS Name | Endpoints Used | Breakdown |
| :--- | :--- | :---: | :--- |
| 🔹 **Control Endpoint** | `EP0` | **1** | Shared bidirectional control endpoint *(whenever ≥1 function is active)* |
| ⌨️ **HID Keyboard** | `hid.usb0` | **1** | 1 Interrupt IN *(LED output reports use `EP0`)* |
| 🖱️ **HID Mouse** | `hid.usb1` | **1** | 1 Interrupt IN |
| 💾 **Mass Storage (UMS)** | `mass_storage.usb0` | **2** | 1 Bulk IN + 1 Bulk OUT |
| 🌐 **USB Ethernet (RNDIS)** | `rndis.usb0` | **3** | 1 Interrupt IN + 1 Bulk IN + 1 Bulk OUT |
| 🌐 **USB Ethernet (CDC ECM)** | `ecm.usb0` | **3** | 1 Interrupt IN + 1 Bulk IN + 1 Bulk OUT |
| 🔌 **USB Serial (CDC ACM)** | `acm.usb0` | **3** | 1 Interrupt IN + 1 Bulk IN + 1 Bulk OUT |

---

### 📋 Example Deployment Combinations on Raspberry Pi (`dwc2` Max: 7)

- ✅ **Keyboard + Mouse + Mass Storage**:
  `1 (EP0) + 1 (Kbd) + 1 (Mouse) + 2 (UMS) = 5 / 7 Endpoints`
- ✅ **Keyboard + Mouse + CDC ECM (or RNDIS)**:
  `1 (EP0) + 1 (Kbd) + 1 (Mouse) + 3 (ECM) = 6 / 7 Endpoints`
- ✅ **Keyboard + Mass Storage + RNDIS (or Serial)**:
  `1 (EP0) + 1 (Kbd) + 2 (UMS) + 3 (RNDIS) = 7 / 7 Endpoints`
- ✅ **RNDIS + CDC ECM Dual-OS Network**:
  `1 (EP0) + 3 (RNDIS) + 3 (ECM) = 7 / 7 Endpoints`
- ❌ **Keyboard + RNDIS + CDC ECM simultaneously**:
  `1 (EP0) + 1 (Kbd) + 3 (RNDIS) + 3 (ECM) = 8 / 7 Endpoints` *(Exceeds `dwc2` budget of 7; select either `RNDIS` for Windows or `ECM` for Linux/macOS, or use a `dwc3` SBC)*
