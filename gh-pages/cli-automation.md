# 🛠️ Command Line Interface & Headless Automation

While Raspiducky includes an embedded web dashboard, its single Go binary works seamlessly from the terminal, automated scripts, `crontab` boot hooks, and `systemd` services.

---

## 💻 CLI Commands & Flags Reference

```bash
raspiducky <subcommand> [flags]
```

| Subcommand | Aliases | Description |
| :--- | :--- | :--- |
| `daemon` | `server` | Starts the background server, REST/WebSocket API, and embedded Web Dashboard. |
| `run <file>` | `exec <file>` | Executes a DuckyScript (`.txt`, `.ducky`) or JavaScript (`.js`) payload file. |
| `version` | `-v`, `--version` | Prints the compiled Raspiducky version string. |
| `help` | `-h`, `--help` | Prints CLI usage and flag help. |

---

### 1. `daemon` Subcommand

Starts the background HTTP/WebSocket server, restores or initializes USB ConfigFS gadget state, and serves the Web Dashboard.

```bash
sudo raspiducky daemon [flags]
```

#### Supported Flags

| Flag | Default | Description |
| :--- | :--- | :--- |
| `-port` | `:8000` | Listen address and port (e.g., `:8000` or `127.0.0.1:8080`). |
| `-storage` | `/var/lib/raspiducky` | Persistent storage directory for `gadget_config.json`, `storage.img`, and saved `scripts/`. |
| `-layout` | *(empty — uses saved/`US`)* | Optional override for keyboard layout (`US`, `ES`, `DE`, `FR`). Leaving it unset preserves layout changes saved in the Web Dashboard across restarts. |
| `-delay` | `-1` *(uses saved/`10ms`)* | Optional override for inter-keystroke delay in ms (use `-delay 0` for raw burst speed, or `-delay 10` for universal PC/console pacing). |

#### Example Usage

```bash
# Start daemon using persisted layout/delay from /var/lib/raspiducky
sudo raspiducky daemon -port :8000 -storage /var/lib/raspiducky

# Force Spanish layout and 0ms burst typing delay at startup
sudo raspiducky daemon -port :8080 -layout ES -delay 0
```

---

### 2. `run` Subcommand (with Smart Daemon Delegation)

Executes a DuckyScript (`.txt`, `.ducky`) or JavaScript (`.js`) payload from the command line. Flags may be placed either before or after `<script_path>`.

```bash
sudo raspiducky run <script_path> [flags]
```

#### Supported Flags

| Flag | Default | Description |
| :--- | :--- | :--- |
| `-layout` | *(empty — uses daemon/`US`)* | Override keyboard translation layout for this run (`US`, `ES`, `DE`, `FR`). |
| `-delay` | `-1` *(uses daemon/`10ms`)* | Override inter-keystroke delay in ms for this run (`0` explicitly selects `0ms`). |
| `-server` | `http://127.0.0.1:8000` | Base URL of a running local `raspiducky daemon` instance. Pass `-server ""` to force standalone direct `/dev/hidg*` access. |

!!! info "Automatic Daemon Delegation"
    When `raspiducky.service` is active on `http://127.0.0.1:8000`, `raspiducky run` automatically submits the script via `POST /api/run`, polls `GET /api/job` until completion, and forwards `Ctrl+C` (`SIGINT`/`SIGTERM`) to `POST /api/stop`. This prevents two processes from competing over `/dev/hidg0` and streams live execution logs to open Web Dashboard tabs. If no daemon is reachable, `raspiducky run` transparently falls back to direct local HID execution.

#### Example Usage

```bash
# Run DuckyScript payload with Spanish layout (delegates to daemon if running)
sudo raspiducky run /var/lib/raspiducky/scripts/payload.txt -layout ES

# Run JavaScript payload at maximum raw speed (0ms delay)
sudo raspiducky run /var/lib/raspiducky/scripts/fast.js -delay 0
```

---

## 🤖 Headless Boot Execution via `crontab`

To run a payload automatically after boot, add an `@reboot` entry to `root`'s `crontab`:

```bash
sudo crontab -e
```

```cron
@reboot sleep 5 && /usr/local/bin/raspiducky run /var/lib/raspiducky/boot_payload.txt -layout ES >> /var/log/raspiducky-boot.log 2>&1
```

---

## ⚙️ Systemd Background Service Setup

The automated `install.sh` script creates `/etc/systemd/system/raspiducky.service` ordered after both ConfigFS and DebugFS mounts, and omits `-layout` so keyboard layouts and typing speeds chosen in the Web Dashboard persist cleanly across reboots:

```ini
[Unit]
Description=Raspiducky USB Gadget & DuckyScript Appliance Daemon
After=network.target sys-kernel-config.mount sys-kernel-debug.mount

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/raspiducky daemon -port :8000 -storage /var/lib/raspiducky
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### Managing the Service

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now raspiducky.service
sudo systemctl status raspiducky.service
sudo journalctl -u raspiducky.service -f
```
