# 🌐 Web Dashboard & REST/WebSocket API Reference

Raspiducky embeds a lightweight, hardened web application directly into its single Go binary using `go:embed`. Users can monitor hardware state, reconfigure composite USB Gadget profiles without rebooting, tune typing speed and keyboard layout, edit payloads, and stream execution logs in real time.

---

## 🎨 Single-Page Application (SPA) Features

* **Composite Gadget Profile Manager**: Independently toggle HID Keyboard, HID Mouse, Mass Storage (Flash Drive or CD-ROM, writable or read-only), Windows RNDIS, Linux/macOS CDC ECM, and CDC ACM Serial interfaces at runtime.
* **Live Hardware Endpoint Guard**: Displays `usedEndpoints / maxEndpoints` directly from kernel DebugFS (`hw_params`) and blocks invalid combinations before touching ConfigFS.
* **Typing Speed & Layout Controls**: Select regional keyboard translation (`US`, `ES`, `DE`, `FR`) and inter-keystroke delay (`10ms` default for universal PC & 60Hz console stability, or `0ms` for raw burst speed).
* **Payload Editor & Library**: Create, edit, run, and persist DuckyScript (`.ducky` / `.txt`) and JavaScript (`.js`) payloads in the browser.
* **Live Execution Console & Instant Stop**: Streams real-time logs over WebSocket and interrupts active scripts and `/dev/hidg*` writes immediately via `Stop`.
* **Host Keyboard LED Indicators**: Reflects live `NumLock`, `CapsLock`, and `ScrollLock` status reported by the connected USB host.

---

## 🔒 Built-In Web & API Security

* **Content-Security-Policy & Frame Protection**: Static UI assets are served with `Content-Security-Policy` (`default-src 'self'`, `object-src 'none'`, `frame-ancestors 'none'`), `X-Frame-Options: DENY`, and `X-Content-Type-Options: nosniff`.
* **Same-Origin / CSRF Protection**: Mutating `POST` and `DELETE` routes validate `Sec-Fetch-Site` and `Origin` headers against the target `Host`, blocking cross-origin browser attacks (`403 Forbidden`) while allowing local CLI / `curl` automation (which omits browser `Origin` headers).
* **Bounded Request Bodies**: JSON request bodies are capped at `1 MiB` via `http.MaxBytesReader`.

---

## 🔌 Real-Time WebSocket API (`/api/ws`)

The WebSocket endpoint streams live state transitions and execution logs, and delivers an atomic initial snapshot (`led_state`, `gadget_status`, and any active `job_status`) immediately upon connection or reconnect:

```text
ws://<raspiducky-ip>:8000/api/ws
```

### Event Envelope

```json
{
  "type": "log | led_state | gadget_status | job_status",
  "level": "INFO | WARN | ERROR",
  "source": "ENGINE | JS | DUCKY | GADGET",
  "message": "Human-readable event message",
  "payload": {}
}
```

### Event Types

#### 1. Log Broadcast (`type: "log"`)
```json
{
  "type": "log",
  "level": "INFO",
  "source": "JS",
  "message": "Stage 1 sequence completed"
}
```

#### 2. Host LED State (`type: "led_state"`)
```json
{
  "type": "led_state",
  "payload": {
    "numLock": true,
    "capsLock": false,
    "scrollLock": false
  }
}
```

#### 3. Gadget Status (`type: "gadget_status"`)
```json
{
  "type": "gadget_status",
  "payload": {
    "deployed": true,
    "activeFunctions": ["hid.usb0", "hid.usb1"],
    "udc": "20980000.usb",
    "maxEndpoints": 7,
    "usedEndpoints": 3,
    "endpointCosts": {
      "ep0": 1,
      "keyboard": 1,
      "mouse": 1,
      "storage": 2,
      "rndis": 3,
      "ecm": 3,
      "serial": 3
    },
    "config": {
      "keyboard": true,
      "mouse": true,
      "storage": false,
      "storageCdrom": false,
      "storageReadOnly": false,
      "rndis": false,
      "ecm": false,
      "serial": false,
      "vendorId": "0x1d6b",
      "productId": "0x0104",
      "manufacturer": "Raspiducky",
      "product": "USB Composite Gadget",
      "serialNumber": "RPD-0001",
      "storageSizeMb": 64,
      "keyboardLayout": "US",
      "typingDelayMs": 10
    }
  }
}
```

#### 4. Job Execution Status (`type: "job_status"`)
`status` transitions through `"running"`, `"completed"`, `"stopped"`, or `"failed"`:
```json
{
  "type": "job_status",
  "payload": {
    "id": "job-1762817000123",
    "name": "recon_payload.js",
    "type": "javascript",
    "status": "running",
    "startedAt": "2026-11-11T00:10:00Z"
  }
}
```

---

## 📡 REST API Reference

| Endpoint | Method | Description |
| :--- | :--- | :--- |
| [`/api/gadget`](#get-apigadget) | `GET` | Retrieve active USB gadget configuration, UDC, and endpoint usage |
| [`/api/gadget`](#post-apigadget) | `POST` | Validate, persist, and deploy USB gadget parameters |
| [`/api/scripts`](#get-apiscripts) | `GET` | List all saved payload scripts in persistent storage |
| [`/api/scripts`](#post-apiscripts) | `POST` | Save or update a payload script (`201 Created`) |
| [`/api/scripts/{name}`](#delete-apiscriptsname) | `DELETE` | Delete a payload script (`204 No Content`) |
| [`/api/job`](#get-apijob) | `GET` | Retrieve current or most recent script job status |
| [`/api/run`](#post-apirun) | `POST` | Trigger execution of an inline script payload |
| [`/api/stop`](#post-apistop) | `POST` | Immediately interrupt active script execution |

---

### `GET /api/gadget`

Returns the active `GadgetStatus`, including `maxEndpoints`, `usedEndpoints`, `endpointCosts`, and persisted `config`.

#### Response (`200 OK`)

```json
{
  "deployed": true,
  "activeFunctions": ["hid.usb0", "hid.usb1"],
  "udc": "20980000.usb",
  "maxEndpoints": 7,
  "usedEndpoints": 3,
  "endpointCosts": {
    "ep0": 1,
    "keyboard": 1,
    "mouse": 1,
    "storage": 2,
    "rndis": 3,
    "ecm": 3,
    "serial": 3
  },
  "config": {
    "keyboard": true,
    "mouse": true,
    "storage": false,
    "storageCdrom": false,
    "storageReadOnly": false,
    "rndis": false,
    "ecm": false,
    "serial": false,
    "vendorId": "0x1d6b",
    "productId": "0x0104",
    "manufacturer": "Raspiducky",
    "product": "USB Composite Gadget",
    "serialNumber": "RPD-0001",
    "storageSizeMb": 64,
    "keyboardLayout": "US",
    "typingDelayMs": 10
  }
}
```

---

### `POST /api/gadget`

Validates, saves (`gadget_config.json`), and applies new ConfigFS settings. Returns the updated `GadgetStatus`.

#### Request Schema

```json
{
  "keyboard": true,
  "mouse": true,
  "storage": false,
  "storageCdrom": false,
  "storageReadOnly": false,
  "rndis": false,
  "ecm": true,
  "serial": false,
  "vendorId": "0x1d6b",
  "productId": "0x0104",
  "manufacturer": "Custom USB Vendor",
  "product": "Custom Composite Device",
  "serialNumber": "SN-994021",
  "storageSizeMb": 128,
  "keyboardLayout": "ES",
  "typingDelayMs": 10
}
```

#### Validation Rules

* `vendorId` and `productId` must be 4-digit hexadecimal strings prefixed with `0x` (e.g., `"0x1d6b"`).
* `manufacturer`, `product`, and `serialNumber` must be non-empty.
* At least one USB function (`keyboard`, `mouse`, `storage`, `rndis`, `ecm`, or `serial`) must be `true`.
* Total endpoints (`1` for `EP0` plus enabled function costs) must not exceed `maxEndpoints` when reported by the UDC.

---

### `GET /api/scripts`

Lists all saved payload scripts residing in `/var/lib/raspiducky/scripts`.

#### Response (`200 OK`)

```json
[
  {
    "name": "windows_notepad.ducky",
    "type": "duckyscript",
    "content": "DELAY 1000\nGUI r\nDELAY 500\nSTRINGLN notepad.exe\n",
    "updatedAt": "2026-11-11T00:15:30Z"
  },
  {
    "name": "mouse_jiggler.js",
    "type": "javascript",
    "content": "mouseMove(10, 0);\ndelay(200);\nmouseMove(-10, 0);\n",
    "updatedAt": "2026-11-11T00:20:00Z"
  }
]
```

---

### `POST /api/scripts`

Creates or updates a script file in persistent storage. Script filenames are strictly sanitized against path traversal.

#### Request Schema

```json
{
  "name": "payload_test.js",
  "type": "javascript",
  "content": "layout('es');\ntype('¡Hola Mundo!\\n');"
}
```

#### Response (`201 Created`)

```json
{
  "message": "Script saved successfully",
  "name": "payload_test.js"
}
```

---

### `DELETE /api/scripts/{name}`

Deletes a saved script file from persistent storage. Returns `204 No Content` on success or `404 Not Found` if the script does not exist.

---

### `GET /api/job`

Returns the active or most recently completed `JobStatus` object (`200 OK`), or `null` if no job has been executed since daemon startup.

```json
{
  "id": "job-1762817000123",
  "name": "payload_test.js",
  "type": "javascript",
  "status": "completed",
  "startedAt": "2026-11-11T00:25:00Z",
  "finishedAt": "2026-11-11T00:25:02Z"
}
```

---

### `POST /api/run`

Compiles/validates and starts immediate execution of a DuckyScript or JavaScript payload. Optional `layout` and `typingDelayMs` fields override the saved defaults for that run (`"typingDelayMs": 0` explicitly selects `0ms` burst mode). Invalid DuckyScript returns `400 Bad Request` with `line N:` details before starting a job.

#### Request Schema

```json
{
  "name": "custom_job",
  "type": "duckyscript",
  "script": "GUI r\nDELAY 500\nSTRINGLN notepad.exe",
  "layout": "ES",
  "typingDelayMs": 10
}
```

#### Response (`200 OK`)

```json
{
  "jobId": "job-1762817000456",
  "message": "Script execution triggered",
  "status": "running"
}
```

---

### `POST /api/stop`

Immediately cancels the running script context, interrupts Goja execution (`vm.Interrupt`), wakes any blocked `/dev/hidg*` write, and releases held keys/buttons.

#### Response (`200 OK`)

```json
{
  "message": "Script execution job stopped"
}
```

---

## 💻 Remote Automation with `curl`

### 1. Fetch Device & Endpoint Status
```bash
curl -fsSL http://192.168.1.50:8000/api/gadget | jq .
```

### 2. Deploy Keyboard + Mass Storage Profile
```bash
curl -fsSL -X POST http://192.168.1.50:8000/api/gadget \
  -H "Content-Type: application/json" \
  -d '{
    "keyboard": true,
    "mouse": false,
    "storage": true,
    "rndis": false,
    "ecm": false,
    "serial": false,
    "vendorId": "0x05ac",
    "productId": "0x0221",
    "manufacturer": "Apple Inc.",
    "product": "Keyboard & Mass Storage",
    "serialNumber": "APL-88301",
    "storageSizeMb": 128,
    "keyboardLayout": "US",
    "typingDelayMs": 10
  }'
```

### 3. Execute a Payload Remotely (with Explicit `0ms` Burst Delay)
```bash
curl -fsSL -X POST http://192.168.1.50:8000/api/run \
  -H "Content-Type: application/json" \
  -d '{
    "name": "remote_exec",
    "type": "javascript",
    "script": "press(\"GUI R\"); delay(500); type(\"calc.exe\\n\");",
    "layout": "US",
    "typingDelayMs": 0
  }'
```

### 4. Stop Active Execution
```bash
curl -fsSL -X POST http://192.168.1.50:8000/api/stop
```
