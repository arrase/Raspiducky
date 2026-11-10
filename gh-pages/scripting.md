# DuckyScript & JavaScript Engines

Raspiducky features a dual scripting runtime supporting both **DuckyScript 1.0–3.0** (`.txt` / `.ducky`) via a direct line-validating compiler and **ECMAScript 5.1+ JavaScript** (`.js`) via an embedded [Goja](https://github.com/dop251/goja) VM.

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
       |  Direct DuckyScript Compiler  |             |   Goja JS Engine (.js)        |
       |  Validates & Compiles to      |             |   Pure Go ECMAScript 5.1+     |
       |  Native []Instruction Opcodes |             |   Runtime + vm.Interrupt()    |
       +---------------+---------------+             +---------------+---------------+
                       |                                             |
                       +----------------------+----------------------+
                                              |
                                              v
                               +-----------------------------+
                               |  Bounded USB HID Layer      |
                               |  (Keyboard, Mouse, LED)     |
                               +-----------------------------+
```

!!! tip "Guaranteed Cleanup & Instant Stop"
    Whether a script finishes normally, encounters a runtime error, or is aborted mid-run via `POST /api/stop` or `Ctrl+C`, `ScriptEngine` immediately interrupts in-flight sleeps, Goja loops (`vm.Interrupt`), and `/dev/hidg*` writes, then automatically releases any held keyboard keys and mouse buttons.

---

## 🦆 1. Direct DuckyScript Compiler (`.txt` / `.ducky`)

Unlike brittle text transpilers, `CompileDuckyScript` (`pkg/scripting/duckyscript.go`) validates every command, argument, and key combo against the active keyboard layout **at compile time** before sending any USB traffic. Syntax or key errors report exact `line N:` diagnostics immediately.

### Supported Commands

| Command | Example | Description |
| :--- | :--- | :--- |
| `REM` / `#` / `//` | `REM Comment` | Single-line comment (ignored). |
| `REM_BLOCK` … `END_REM` | `REM_BLOCK` | Multi-line DuckyScript 3.0 comment block. |
| `ATTACKMODE` | `ATTACKMODE HID` | Accepted for DuckyScript 3.0 payload compatibility (no-op; configure composite functions via Dashboard/API). |
| `DEFAULT_DELAY` / `DEFAULTDELAY` | `DEFAULT_DELAY 50` | Default post-instruction delay (ms) appended after subsequent repeatable commands. |
| `DELAY` | `DELAY 500` | Pauses execution for the specified milliseconds. |
| `REPEAT` | `REPEAT 5` | Repeats the preceding repeatable instruction `N` times. |
| `LAYOUT` | `LAYOUT ES` | Switches keyboard translation layout (`US`, `ES`, `DE`, `FR`) dynamically mid-script. |
| `TYPING_SPEED` / `STRINGDELAY` / `STRING_DELAY` | `TYPING_SPEED 10 5` | Sets inter-keystroke delay (`ms`) and optional random jitter (`ms`). Use `0` for max raw speed. |
| `STRING` | `STRING whoami` | Types inline text using the active keyboard layout. |
| `STRING` … `END_STRING` | *(Multi-line block)* | Types multi-line text verbatim (preserving indentation and internal newlines) without trailing `ENTER`. |
| `STRINGLN` | `STRINGLN whoami` | Types inline text followed by `ENTER`. |
| `STRINGLN` … `END_STRINGLN` | *(Multi-line block)* | Types multi-line text verbatim followed by a final `ENTER` (also accepts `END_STRING`). |
| `ENTER` | `ENTER` | Presses Return / Enter. |
| `GUI` / `WINDOWS` / `COMMAND` | `GUI r` or `GUI` | Presses Super/Windows/Command alone or as a modifier combo. |
| `CTRL` / `CONTROL` | `CTRL SHIFT ESC` | Key combo with Control modifier. |
| `ALT` / `ALTGR` | `ALT F4` | Key combo with Alt or AltGr modifier. |
| `SHIFT` | `SHIFT TAB` | Key combo with Shift modifier. |
| `MENU` / `APP` | `MENU` | Context menu / Application key. |
| Navigation & Editing Keys | `TAB`, `ESCAPE`, `SPACE`, `BACKSPACE`, `DELETE`, `INSERT`, `HOME`, `END`, `PAGEUP`, `PAGEDOWN`, `UP` / `UPARROW`, `DOWN` / `DOWNARROW`, `LEFT` / `LEFTARROW`, `RIGHT` / `RIGHTARROW` | Standard navigation and editing keys. |
| Function & Lock Keys | `F1`–`F12`, `CAPSLOCK`, `NUMLOCK`, `SCROLLLOCK`, `PRINTSCREEN`, `PAUSE` / `BREAK` | Function and lock toggle keys. |
| `HOLD` | `HOLD SHIFT` | Holds down one or more keys without releasing until `RELEASE` or script exit. |
| `RELEASE` | `RELEASE SHIFT` | Releases previously held key(s). |
| `MOUSE_MOVE` | `MOUSE_MOVE 400 -150` | Relative mouse move (automatically splits deltas `> ±127` across multiple HID reports). |
| `MOUSE_MOVE_ABS` / `MOUSE_MOVETO` | `MOUSE_MOVETO 16384 16384` | Moves cursor to normalized 15-bit screen coordinates (`0..32767`). |
| `MOUSE_CLICK` | `MOUSE_CLICK left` | Clicks `left` (default), `right`, or `middle` mouse button. |
| `WAIT_LED` | `WAIT_LED NUM 10000` | Waits for a host LED change (`ANY`, `NUM`, `CAPS`, `SCROLL`) with optional timeout in ms (default `5000`). |

### DuckyScript Example (with Multi-Line Blocks & Host Sync)

```duckyscript
REM Wait up to 15s for host user to toggle CapsLock, then type multi-line script
LAYOUT US
TYPING_SPEED 10 0
WAIT_LED CAPS 15000

GUI r
DELAY 500
STRINGLN notepad.exe
DELAY 800

STRINGLN
Hello from Raspiducky!
Multi-line DuckyScript 3.0 blocks preserve newlines cleanly.
END_STRINGLN
```

---

## ⏱️ Typing Speed & 60Hz Host Compatibility

Raspiducky decouples physical key hold duration from inter-keystroke pacing so typing stays both fast and reliable across desktop OSes and frame-sampled console hosts (such as 60Hz game consoles):

* **Default Pacing (`10ms` delay)**: Uses a crisp `5–20ms` key hold plus `10ms` inter-key gap, and applies a dedicated `20ms` `KeyEnter` hold + `25ms` post-newline settle so editors on 60Hz hosts never drop newlines or swallow the first character of the next line.
* **Maximum Raw Speed (`0ms` delay)**: Uses a minimal `2ms` key hold with zero inter-key sleep for maximum burst throughput on standard PCs.
* **CRLF & Unicode Resilience**: `\r\n` line endings are normalized to `\n`, typographic quotes/dashes/non-breaking spaces are mapped cleanly, dead keys (`^`, `` ` ``, `´`, `~`) emit `dead-key + Space` automatically, and unsupported control/format characters are skipped rather than aborting the payload.

---

## ⚡ 2. JavaScript HIDScript Engine (`.js`)

Powered by [Goja](https://github.com/dop251/goja), the JavaScript engine provides full programmatic control: variables, loops, conditionals, functions, `try`/`catch` error handling, and interactive host LED synchronization.

### Keyboard Functions

#### `type(text: string): void`
Types the provided string character by character using the active keyboard layout.
```javascript
type("Hello World!\n");
```

#### `press(keys: string): void`
Presses and releases a single key or space-separated modifier combination.
```javascript
press("GUI R");
press("CTRL ALT DELETE");
press("SHIFT TAB");
```

#### `hold(keys: string): void` & `release(keys: string): void`
Depresses or releases one or more keys across subsequent operations. Any keys still held when the script ends are released automatically.
```javascript
hold("SHIFT");
press("DOWNARROW");
release("SHIFT");
```

#### `layout(name: string): void`
Changes the active keyboard translation table (`"us"`, `"es"`, `"de"`, `"fr"`).
```javascript
layout("es");
type("¿Hola, cómo estás?\n");
```

#### `typingSpeed(delayMs: number, jitterMs?: number): void`
Sets the inter-keystroke delay (`>= 0`) and optional random jitter in milliseconds.
```javascript
typingSpeed(0, 0);   // Maximum raw burst speed
typingSpeed(20, 10); // Human-like pacing (20ms + 0..10ms jitter)
```

---

### Timing & Logging Functions

#### `delay(ms: number): void`
Pauses execution for `ms` milliseconds (instantly interruptible on `Stop`).
```javascript
delay(1000);
```

#### `log(...args: any[]): void` / `console.log(...args: any[]): void`
Streams log lines in real time to CLI stdout and connected WebSocket dashboard clients (`console.warn` and `console.error` are also bound).
```javascript
log("Stage 1 complete");
console.log("Target layout:", "ES");
```

---

### Mouse Functions

#### `mouseMove(dx: number, dy: number): void`
Moves the mouse pointer by relative `dx`, `dy` pixels. Displacements outside `-127..127` are automatically split into sequential HID reports.
```javascript
mouseMove(350, -120);
```

#### `mouseMoveTo(x: number, y: number): void`
Moves the cursor to absolute normalized coordinates (`0` to `32767` on X and Y).
```javascript
mouseMoveTo(16384, 16384); // Center of screen
```

#### `mouseClick(button?: string): void`
Clicks `"left"` (default), `"right"`, or `"middle"`.
```javascript
mouseClick("left");
mouseClick("right");
```

---

### Host Synchronization & LED Listener

#### `waitLED(filter?: string, timeoutMs?: number): LEDState`
Blocks until the host sends a keyboard LED report matching `filter`, or throws on timeout.

* **Filter**: `"ANY"` (default), `"NUM"` / `"NUMLOCK"`, `"CAPS"` / `"CAPSLOCK"`, `"SCROLL"` / `"SCROLLLOCK"`.
* **Timeout**: Milliseconds (default `5000`).
* **Returns**: `{ numLock: boolean, capsLock: boolean, scrollLock: boolean }`.

```javascript
log("Waiting up to 15s for NumLock toggle on host...");
try {
  var state = waitLED("NUM", 15000);
  log("NumLock is now:", state.numLock);
  press("GUI R");
  delay(300);
  type("calc.exe\n");
} catch (err) {
  log("Timed out waiting for LED:", err);
}
```

---

## 💡 Host LED Listener Technical Operation

When a host toggles `NumLock`, `CapsLock`, or `ScrollLock`, the host OS sends a 1-byte HID `SET_REPORT` output transfer over `EP0` (`no_out_endpoint=1`), which the kernel `f_hid` driver delivers to `/dev/hidg0`:

1. **Output Report Bitmask**:
   - Bit 0 (`0x01`): `Num Lock`
   - Bit 1 (`0x02`): `Caps Lock`
   - Bit 2 (`0x04`): `Scroll Lock`
2. **Event Fan-Out**: `LEDWatcher` (`pkg/hid/led.go`) polls `/dev/hidg0` and broadcasts state transitions to active `waitLED()` / `WAIT_LED` callers and connected WebSocket clients.
