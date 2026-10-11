# 🛠️ Command Line Interface & Headless Automation

While Raspiducky includes an embedded web dashboard, its core architecture is built around a single Go binary that functions seamlessly in command line environments, headless boot setups, and automated background system services.

---

## 💻 CLI Commands & Flags Reference

The `raspiducky` CLI binary supports two primary operating subcommands: `daemon` (starts the server engine and web interface) and `run` (executes payload scripts directly).

```bash
raspiducky [subcommand] [flags]
```

### 1. `daemon` Subcommand

Starts the background server, REST/WebSocket API endpoints, and web dashboard UI.

```bash
sudo raspiducky daemon [flags]
```

#### Supported Flags

| Flag | Default Value | Description |
| :--- | :--- | :--- |
| `-port` | `:8000` | Network binding interface and HTTP/WebSocket port. |
| `-storage` | `/var/lib/raspiducky` | Path to persistent storage directory for disk images, saved scripts, and gadget configurations. |
| `-layout` | `US` | Default HID keyboard translation layout (`US`, `ES`, `DE`, `FR`). |

#### Example Usage

```bash
# Start daemon on port 8080 using Spanish keyboard layout
sudo raspiducky daemon -port :8080 -layout ES -storage /etc/raspiducky
```

---

### 2. `run` Subcommand

Executes a single payload script directly from the terminal without initiating the HTTP server or web UI. This subcommand accepts both DuckyScript (`.txt`, `.ducky`) and JavaScript (`.js`) files.

```bash
sudo raspiducky run <script_path> [flags]
```

#### Supported Flags

| Flag | Default Value | Description |
| :--- | :--- | :--- |
| `-layout` | `US` | HID keyboard translation layout for character encoding (`US`, `ES`, `DE`, `FR`). |

#### Example Usage

```bash
# Execute a DuckyScript payload with Spanish layout
sudo raspiducky run /var/lib/raspiducky/payload.txt -layout ES

# Execute a JavaScript payload
sudo raspiducky run /var/lib/raspiducky/jiggler.js
```

---

## 🤖 Headless Boot Execution via `crontab`

For standalone, headless operations where Raspiducky must automatically run a specific payload as soon as the SBC boots up, you can schedule execution using the root user's `crontab`.

### Configuration Steps

1. Open the root user's crontab editor:

```bash
sudo crontab -e
```

2. Add an `@reboot` entry pointing to the target script:

```cron
@reboot /usr/local/bin/raspiducky run /var/lib/raspiducky/boot_payload.txt -layout ES >> /var/log/raspiducky-boot.log 2>&1
```

3. Save and exit. On subsequent boots, the script will execute automatically without human intervention.

---

## ⚙️ Systemd Background Service Setup

To run Raspiducky as a persistent system daemon that starts on boot and automatically restarts if interrupted, use a systemd service unit.

### Service Unit File (`/etc/systemd/system/raspiducky.service`)

Create the systemd service file with the following contents:

```ini
[Unit]
Description=Raspiducky USB Gadget & DuckyScript Appliance Daemon
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/raspiducky daemon -port :8000 -storage /var/lib/raspiducky -layout US
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### Enabling & Managing the Service

Load the service unit, enable auto-start on boot, and manage execution using standard `systemctl` commands:

```bash
# Reload systemd manager configuration
sudo systemctl daemon-reload

# Enable service to run on boot
sudo systemctl enable raspiducky.service

# Start the service immediately
sudo systemctl start raspiducky.service

# Check live service status
sudo systemctl status raspiducky.service

# View real-time application logs
sudo journalctl -u raspiducky.service -f
```
