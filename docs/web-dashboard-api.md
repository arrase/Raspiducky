# 🌐 Web Dashboard & REST/WebSocket API Reference

Raspiducky embeds a lightweight, high-performance web application directly into its single Go binary using `go:embed`. This allows users to monitor device state, configure USB Gadget parameters on the fly, edit payloads, and stream execution logs in real time from any browser—without external web servers or static assets.

---

## 🎨 Single-Page Application (SPA) Features

The embedded Single-Page Application features a modern dark-mode user interface designed for low-latency hardware management and execution feedback.

* **Gadget Profile Manager**: Enable or disable HID Keyboard, HID Mouse, Mass Storage (UMS), USB Ethernet (RNDIS/ECM), and Serial (CDC ACM) interfaces dynamically without rebooting.
* **Endpoint Limit Guard**: Automatically reads kernel DebugFS endpoint capabilities and prevents pushing configurations that exceed board hardware limits.
* **Payload Editor & Library**: Write, edit, test, and save DuckyScript (`.ducky`/`.txt`) and JavaScript (`.js`) scripts directly in the browser.
* **Live Execution Console**: Interactive job runner with real-time log output and stop controls.
* **Host Keyboard LED Sync**: Displays real-time status of host target keyboard LEDs (NumLock, CapsLock, ScrollLock).

---

## 🔌 Real-Time WebSocket API (`/api/ws`)

The WebSocket endpoint provides a full-duplex communication channel for real-time state updates and streaming logs.

```text
ws://<raspiducky-ip>:8000/api/ws
```

### Event Message Format

All WebSocket messages adhere to the following JSON structure:

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
Emitted whenever the scripting engine, JavaScript console, or system background tasks write log output.

```json
{
  "type": "log",
  "level": "INFO",
  "source": "JS",
  "message": "Type sequence completed successfully"
}
```

#### 2. Gadget Status (`type: "gadget_status"`)
Emitted whenever the active USB gadget profile is reconfigured or deployed.

```json
{
  "type": "gadget_status",
  "payload": {
    "deployed": true,
    "activeFunctions": ["hid.usb0", "hid.usb1"],
    "udc": "20980000.usb",
    "maxEndpoints": 7,
    "config": {
      "keyboard": true,
      "mouse": true,
      "storage": false,
      "ethernet": false,
      "serial": false,
      "vendorId": "0x1d6b",
      "productId": "0x0104",
      "manufacturer": "Raspiducky Labs",
      "product": "Raspiducky Multi-Function HID",
      "serialNumber": "RPD-2026-0001",
      "storageSizeMb": 100,
      "keyboardLayout": "US"
    }
  }
}
```

#### 3. Job Execution Status (`type: "job_status"`)
Emitted when a script job starts, finishes, fails, or is stopped.

```json
{
  "type": "job_status",
  "payload": {
    "id": "job-84920",
    "name": "recon_payload.js",
    "type": "javascript",
    "status": "running",
    "startedAt": "2026-08-06T02:00:00Z"
  }
}
```

---

## 📡 REST API Reference

| Endpoint | Method | Description |
| :--- | :--- | :--- |
| [`/api/gadget`](#get-apigadget) | `GET` | Retrieve active USB gadget configuration & hardware limits |
| [`/api/gadget`](#post-apigadget) | `POST` | Update and deploy new USB gadget parameters |
| [`/api/scripts`](#get-apiscripts) | `GET` | List all saved script templates in persistent storage |
| [`/api/scripts`](#post-apiscripts) | `POST` | Save or update a script template |
| [`/api/scripts/{name}`](#delete-apiscriptsname) | `DELETE` | Delete a script template from disk |
| [`/api/run`](#post-apirun) | `POST` | Trigger execution of an inline or saved script |
| [`/api/stop`](#post-apistop) | `POST` | Stop active script execution |

---

### `GET /api/gadget`

Retrieves current USB gadget state, configured functions, active UDC hardware controller, and endpoint limits.

#### Response (`200 OK`)

```json
{
  "deployed": true,
  "activeFunctions": [
    "hid.usb0",
    "hid.usb1"
  ],
  "udc": "20980000.usb",
  "maxEndpoints": 7,
  "config": {
    "keyboard": true,
    "mouse": true,
    "storage": false,
    "ethernet": false,
    "serial": false,
    "vendorId": "0x1d6b",
    "productId": "0x0104",
    "manufacturer": "Raspiducky Labs",
    "product": "Raspiducky Multi-Function HID",
    "serialNumber": "RPD-2026-0001",
    "storageSizeMb": 100,
    "keyboardLayout": "US"
  }
}
```

---

### `POST /api/gadget`

Updates and instantly deploys new ConfigFS settings. Returns updated `GadgetStatus`.

#### Request Schema

```json
{
  "keyboard": true,
  "mouse": true,
  "storage": false,
  "ethernet": false,
  "serial": false,
  "vendorId": "0x1d6b",
  "productId": "0x0104",
  "manufacturer": "Custom USB Vendor",
  "product": "Custom Device Name",
  "serialNumber": "SN-994021",
  "storageSizeMb": 100,
  "keyboardLayout": "ES"
}
```

#### Validation Rules

* `vendorId` and `productId` must be hexadecimal strings prefixed with `0x`.
* At least one USB function (`keyboard`, `mouse`, `storage`, `ethernet`, or `serial`) must be set to `true`.
* Total endpoints consumed by enabled functions must not exceed `maxEndpoints`.

---

### `GET /api/scripts`

Lists all saved payload scripts residing in the storage directory.

#### Response (`200 OK`)

```json
[
  {
    "name": "windows_reverse_shell.ducky",
    "type": "duckyscript",
    "content": "DELAY 1000\nGUI r\nSTRING powershell.exe\nENTER\n",
    "description": "Saved duckyscript payload",
    "updatedAt": "2026-08-06T01:15:30Z"
  },
  {
    "name": "mouse_jiggler.js",
    "type": "javascript",
    "content": "mouseMove(10, 0);\ndelay(200);\n",
    "description": "Saved javascript payload",
    "updatedAt": "2026-08-06T01:20:00Z"
  }
]
```

---

### `POST /api/scripts`

Creates or updates a script template file in persistent storage.

#### Request Schema

```json
{
  "name": "payload_test.js",
  "type": "javascript",
  "content": "layout('es');\ntype('Hello World!\\n');",
  "description": "Testing payload for Spanish layout target"
}
```

---

### `DELETE /api/scripts/{name}`

Deletes a script file from persistent storage.

#### Response (`200 OK`)

```json
{
  "status": "success",
  "message": "Script 'payload_test.js' deleted"
}
```

---

### `POST /api/run`

Triggers immediate execution of a script (DuckyScript or JavaScript).

#### Request Schema

```json
{
  "name": "custom_job",
  "type": "javascript",
  "script": "press('GUI R'); delay(500); type('notepad.exe\\n');"
}
```

#### Response (`200 OK`)

```json
{
  "id": "job-48201",
  "name": "custom_job",
  "type": "javascript",
  "status": "running",
  "startedAt": "2026-08-06T02:50:00Z"
}
```

---

### `POST /api/stop`

Stops the currently running script job.

#### Response (`200 OK`)

```json
{
  "status": "success",
  "message": "Active job stopped"
}
```

---

## 💻 Remote Automation with `curl`

You can easily trigger and control Raspiducky remotely using `curl` commands from your terminal or automation scripts.

### 1. Fetch Device Status
```bash
curl -fsSL http://192.168.1.50:8000/api/gadget | jq .
```

### 2. Deploy Keyboard & Mass Storage Profile
```bash
curl -X POST http://192.168.1.50:8000/api/gadget \
  -H "Content-Type: application/json" \
  -d '{
    "keyboard": true,
    "mouse": false,
    "storage": true,
    "ethernet": false,
    "serial": false,
    "vendorId": "0x05ac",
    "productId": "0x0221",
    "manufacturer": "Apple Inc.",
    "product": "Keyboard & Mass Storage",
    "serialNumber": "APL-88301",
    "storageSizeMb": 250,
    "keyboardLayout": "US"
  }'
```

### 3. Save a DuckyScript Payload
```bash
curl -X POST http://192.168.1.50:8000/api/scripts \
  -H "Content-Type: application/json" \
  -d '{
    "name": "hello_remote.ducky",
    "type": "duckyscript",
    "content": "DELAY 1000\nGUI r\nDELAY 500\nSTRING notepad.exe\nENTER\nSTRING Remote execution via cURL!\nENTER\n"
  }'
```

### 4. Execute Payload Remotely
```bash
curl -X POST http://192.168.1.50:8000/api/run \
  -H "Content-Type: application/json" \
  -d '{
    "name": "remote_exec",
    "type": "javascript",
    "script": "layout(\"us\"); press(\"GUI R\"); delay(500); type(\"calc.exe\\n\");"
  }'
```

### 5. Stop Execution
```bash
curl -X POST http://192.168.1.50:8000/api/stop
```
