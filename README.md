# Raspiducky 🦆⚡

**Raspiducky** is a modern, high-performance, lightweight USB Gadget and DuckyScript/HIDScript execution appliance for Raspberry Pi Zero W (and similar boards supporting USB OTG / USB host mode). 

It is designed as a complete, modern rewrite of the legacy `P4wnP1 A.L.O.A.` project, eliminating heavy dependencies (like Kali Linux) in favor of running as a **single Go executable** on standard, lightweight Linux distributions such as **Raspberry Pi OS Lite**.

---

## 🌟 Key Features

* **Single Executable Binary**: Built in modern Golang, incorporating an embedded REST API, WebSocket server, CLI commands, and single-page web dashboard using `go:embed`.
* **Plug & Play Linux USB ConfigFS**: Direct interaction with `/sys/kernel/config/usb_gadget` for runtime configuration without system reboots:
  * **HID Keyboard**: 8-byte reports with multi-language layout translation (**US**, **ES**, **DE**, **FR**).
  * **HID Mouse**: Relative mouse movements, absolute positioning, and button clicks.
  * **USB Mass Storage (UMS)**: Mounting ISO or RAW disk image backing files in CD-ROM or writable Flash Drive modes.
  * **USB Network**: CDC ECM (Linux/Mac) and RNDIS (Windows) with custom MAC addresses and Windows OS descriptors.
  * **USB Serial**: CDC ACM serial device interface.
* **Dual Scripting Engines**:
  * **JavaScript Engine (Goja)**: Pure Go ECMAScript 5.1+ runtime with native USB HID bindings (`type()`, `press()`, `delay()`, `layout()`, `typingSpeed()`, `waitLED()`, `mouseMove()`, `mouseClick()`).
  * **DuckyScript Parser**: Transparently parses and executes standard Rubber Ducky `.txt` scripts.
* **Host Keyboard LED State Listener**: Reads output report updates from `/dev/hidg0` (NUMLOCK, CAPSLOCK, SCROLLLOCK) enabling synchronization and payload triggering based on target host interactions.
* **Modern Embedded Web Dashboard**: Sleek, dark-mode single-page dashboard with real-time WebSocket log streaming, payload editor, job manager, and gadget controls.

---

## 🏗️ Architecture

```
                                  +---------------------------------------+
                                  |         Web Browser / REST / WS       |
                                  +-------------------+-------------------+
                                                      |
                                                      v
+---------------------------------------------------------------------------------------------------+
| Raspiducky (Single Executable)                                                                    |
|                                                                                                   |
|  +-------------------+   +--------------------+   +-----------------------+   +----------------+  |
|  | Embedded Web UI   |   |   REST & WS API    |   |  Goja JS Engine &     |   | Storage        |  |
|  | (go:embed)        |   |   (pkg/api)        |   |  DuckyScript Parser   |   | (pkg/storage)  |  |
|  +-------------------+   +---------+----------+   +-----------+-----------+   +----------------+  |
|                                    |                      |                                       |
|                                    v                      v                                       |
|                         +-----------------------------------+                                     |
|                         |    HID & ConfigFS Controllers     |                                     |
|                         |    (pkg/gadget, pkg/hid)          |                                     |
|                         +-----------------+-----------------+                                     |
+-------------------------------------------|-------------------------------------------------------+
                                            |
                                            v
                      +-------------------------------------------+
                      | Linux Kernel ConfigFS & /dev/hidgX        |
                      +-------------------------------------------+
```

---

## 🚀 Getting Started

### Prerequisites
* Raspberry Pi Zero W / Zero 2 W / Pi 4 (or any board running Linux with `configfs` and USB OTG UDC support).
* Operating System: **Raspberry Pi OS Lite** (recommended) or Alpine Linux.

### Building from Source

#### 1. Native Build
```bash
git clone https://github.com/arrase/Raspiducky.git
cd Raspiducky
go build -o build/raspiducky ./cmd/raspiducky
```

#### 2. Cross-Compiling for Raspberry Pi Zero W (ARMv6)
```bash
GOOS=linux GOARCH=arm GOARM=6 go build -ldflags="-s -w" -o build/raspiducky ./cmd/raspiducky
```

---

## 🎮 Usage

### 1. Starting the Daemon & Web Dashboard
Run `raspiducky` in daemon mode with root privileges (required to interact with `/sys/kernel/config/usb_gadget` and `/dev/hidg*`):

```bash
sudo ./raspiducky daemon -port :8000
```
Open your browser and navigate to `http://<raspberry-pi-ip>:8000` to access the live dashboard.

### 2. Executing Scripts via CLI
You can execute a DuckyScript (`.txt`) or JavaScript (`.js`) file directly from the command line:

```bash
sudo ./raspiducky run payloads/hello_world.txt
```

---

## 📜 Scripting Examples

### DuckyScript Example (`hello.txt`)
```duckyscript
REM Standard Rubber Ducky Payload
DELAY 1000
GUI r
DELAY 500
STRING notepad.exe
ENTER
DELAY 1000
STRING Hello World from Raspiducky!
ENTER
```

### JavaScript HIDScript Example (`hello.js`)
```javascript
// Set keyboard layout to Spanish and speed
layout("es");
typingSpeed(20, 50);

// Open Run dialog on Windows
press("GUI R");
delay(500);
type("notepad.exe\n");
delay(1000);

// Type text
type("Hello from Raspiducky JS Engine!\n");

// Wait for target user to hit NumLock key
waitLED("NUM", 10000);
type("NumLock was toggled on host!\n");
```

---

## 🛠️ REST API Reference

| Endpoint | Method | Description |
| :--- | :--- | :--- |
| `/api/gadget` | `GET` | Retrieve active USB gadget configuration & state |
| `/api/gadget` | `POST` | Update and deploy new USB gadget parameters |
| `/api/scripts` | `GET` | List all saved script templates |
| `/api/scripts` | `POST` | Save or update a script template |
| `/api/scripts/{name}` | `DELETE` | Delete a script template |
| `/api/run` | `POST` | Trigger script execution (JS or DuckyScript) |
| `/api/stop` | `POST` | Stop active script execution |
| `/api/ws` | `GET` | Real-time WebSocket connection for logs & status events |

---

## 🧪 Testing

Run all unit tests across the codebase:
```bash
go test -v ./...
```

---

## 📄 License
This project is released under the **MIT License**.
