# DuckyScript & JavaScript Engines

Raspiducky features a dual scripting engine architecture that supports both traditional DuckyScript `.txt` payloads and advanced ECMAScript 5.1+ JavaScript `.js` scripts with native USB HID hardware bindings.

---

## ⚙️ Dual Scripting Engine Architecture

```text
                               +-----------------------------+
                               |     Payload Source Code     |
                               +--------------+--------------+
                                              |
                       +----------------------+----------------------+
                       |                                             |
                       v                                             v
       +-------------------------------+             +-------------------------------+
       |   DuckyScript Parser (.txt)   |             |   Goja JS Engine (.js)        |
       |   Transpiles DuckyScript to   |             |   Pure Go ECMAScript 5.1+     |
       |   JavaScript AST              |             |   Runtime Environment         |
       +---------------+---------------+             +---------------+---------------+
                       |                                             |
                       +----------------------+----------------------+
                                              |
                                              v
                               +-----------------------------+
                               | Native USB HID Bindings     |
                               | (Keyboard, Mouse, LED)      |
                               +-----------------------------+
```

---

## 🦆 1. DuckyScript Parser (`.txt`)

Raspiducky natively parses and executes standard Hak5 Rubber Ducky v1 syntax. When a `.txt` file is selected, the parser converts DuckyScript commands into JavaScript code line-by-line before passing it to the execution engine.

### Supported Commands

| Command | Example | Description |
| :--- | :--- | :--- |
| `REM` | `REM This is a comment` | Comment line (ignored during execution). |
| `DEFAULT_DELAY` / `DEFAULTDELAY` | `DEFAULTDELAY 100` | Sets default delay in milliseconds applied between every subsequent command. |
| `DELAY` | `DELAY 500` | Pauses execution for the specified milliseconds. |
| `STRING` | `STRING powershell.exe` | Types out text string using active keyboard layout. |
| `STRINGLN` | `STRINGLN whoami` | Types out text string followed immediately by an `ENTER` keystroke. |
| `ENTER` | `ENTER` | Presses the Enter / Return key. |
| `GUI` / `WINDOWS` | `GUI r` or `GUI` | Presses Windows/Super key or key combo with GUI modifier. |
| `CTRL` / `CONTROL` | `CTRL c` or `CTRL ALT DELETE` | Key combo with Control modifier. |
| `ALT` | `ALT F4` | Key combo with Alt modifier. |
| `SHIFT` | `SHIFT TAB` | Key combo with Shift modifier. |
| `MENU` / `APP` | `MENU` | Opens context menu (Shift + F10 or App key). |
| `TAB`, `ESCAPE`, `SPACE` | `TAB` | Standard navigation keys. |
| `UP`, `DOWN`, `LEFT`, `RIGHT` | `UPARROW` | Arrow navigation keys. |

### DuckyScript Example

```duckyscript
REM Automated Windows CMD Execution
DEFAULT_DELAY 100
DELAY 1000
GUI r
DELAY 500
STRING cmd.exe
ENTER
DELAY 1000
STRING echo Hello from Raspiducky!
ENTER
```

---

## ⚡ 2. JavaScript HIDScript Engine (`.js`)

Powered by [Goja](https://github.com/dop251/goja) (a pure Go ECMAScript 5.1+ implementation), the JS engine gives payload developers full programmatic control: variables, loops, conditional logic, functions, try/catch blocks, and asynchronous LED event synchronization.

---

## 📖 JavaScript API Reference

The JS execution context exposes the following native functions:

### Keyboard Functions

#### `type(text: string): void`
Types out the provided string character by character according to the currently active keyboard layout.
```javascript
type("Hello World!\n");
```

#### `press(keys: string): void`
Simulates pressing a key combination. Key tokens are separated by spaces.
```javascript
press("GUI R");
press("CTRL ALT DELETE");
press("SHIFT TAB");
```

#### `layout(lang: string): void`
Changes the active keyboard translation table.
* **Supported values**: `"us"`, `"es"`, `"de"`, `"fr"`.
```javascript
layout("es"); // Switch to Spanish layout
type("¿Hola, cómo estás?");
```

#### `typingSpeed(delayMs: number, jitterMs: number): void`
Sets the inter-keystroke delay and optional random jitter (in milliseconds) to simulate human typing patterns.
```javascript
// 30ms delay per key with up to 15ms random jitter
typingSpeed(30, 15);
type("Typing like a human...");
```

---

### Timing & Logging Functions

#### `delay(ms: number): void`
Pauses script execution for the specified number of milliseconds.
```javascript
delay(1000); // Pause for 1 second
```

#### `log(...args: any[]): void` / `console.log(...args: any[]): void`
Writes log messages to the execution console and streams them via WebSocket to the Web Dashboard.
```javascript
log("Executing payload stage 2...");
console.log("Current step completed successfully.");
```

---

### Mouse Functions

#### `mouseMove(dx: number, dy: number): void`
Moves the mouse pointer by relative X and Y offsets (range: `-127` to `127`).
```javascript
mouseMove(50, -20); // Move 50px right, 20px up
```

#### `mouseMoveTo(x: number, y: number): void`
Moves the mouse pointer to absolute screen coordinates (range: `0` to `32767`).
```javascript
mouseMoveTo(16384, 16384); // Move to center of screen
```

#### `mouseClick(button: string): void`
Triggers a mouse button click.
* **Supported values**: `"left"`, `"right"`, `"middle"`.
```javascript
mouseClick("left");
delay(100);
mouseClick("right");
```

---

### Host Synchronization & LED Listener

#### `waitLED(filter: string, timeoutMs: number): LEDState`
Blocks execution until the target host sends a keyboard LED state update over `/dev/hidg0` matching the specified filter mask, or until the timeout expires.

* **Filter Values**: `"NUM"` / `"NUMLOCK"`, `"CAPS"` / `"CAPSLOCK"`, `"SCROLL"` / `"SCROLLLOCK"`, `"ANY"`.
* **Timeout**: Milliseconds (default: `5000` ms).
* **Return Value**: `LEDState` object `{ numLock: boolean, capsLock: boolean, scrollLock: boolean }`.

```javascript
log("Waiting for host user to press NumLock key...");

try {
  // Wait up to 15 seconds for NumLock toggle
  var state = waitLED("NUM", 15000);
  log("NumLock toggled! NumLock active: " + state.numLock);
  
  // Trigger secondary stage payload
  press("GUI R");
  delay(300);
  type("calc.exe\n");
} catch (err) {
  log("Timeout waiting for LED event: " + err);
}
```

---

## 💡 Host LED Listener Technical Operation

When a target host receives keystrokes like `NumLock`, `CapsLock`, or `ScrollLock`, the host operating system sends a USB Output Report back to `/dev/hidg0` to switch the physical keyboard's indicator LEDs.

Raspiducky's `LEDWatcher` (`pkg/hid/led.go`) continuously monitors `/dev/hidg0` via non-blocking read operations:
1. **Host Output Report Byte Structure**:
   - Bit 0 (`0x01`): `Num Lock`
   - Bit 1 (`0x02`): `Caps Lock`
   - Bit 2 (`0x04`): `Scroll Lock`
2. **Event Dispatching**: When a matching bit change occurs, `LEDWatcher` broadcasts the updated state to active subscribers (`waitLED()` listeners in JavaScript and real-time WebSocket clients).
