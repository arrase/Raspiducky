package scripting

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

// ParseDuckyScript parses Rubber Ducky script syntax into equivalent JavaScript code.
func ParseDuckyScript(duckySource string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(duckySource))
	var jsLines []string

	defaultDelay := 0
	var lastJSCommand string

	for scanner.Scan() {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		// Comments
		if line == "REM" || strings.HasPrefix(line, "REM ") {
			comment := strings.TrimPrefix(line, "REM")
			comment = strings.TrimSpace(comment)
			jsLines = append(jsLines, fmt.Sprintf("// %s", comment))
			continue
		}

		// DEFAULT_DELAY / DEFAULTDELAY
		if strings.HasPrefix(line, "DEFAULT_DELAY") || strings.HasPrefix(line, "DEFAULTDELAY") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				if delayVal, err := strconv.Atoi(parts[1]); err == nil {
					defaultDelay = delayVal
					jsLines = append(jsLines, fmt.Sprintf("// Default delay set to %d ms", defaultDelay))
				}
			}
			continue
		}

		// REPEAT
		if strings.HasPrefix(line, "REPEAT") {
			parts := strings.Fields(line)
			if len(parts) >= 2 && lastJSCommand != "" {
				if count, err := strconv.Atoi(parts[1]); err == nil && count > 0 {
					jsLines = append(jsLines, fmt.Sprintf("for (let _i = 0; _i < %d; _i++) { %s }", count, lastJSCommand))
					if defaultDelay > 0 {
						jsLines = append(jsLines, fmt.Sprintf("delay(%d);", defaultDelay))
					}
				}
			}
			continue
		}

		// Parse line into JS statement
		jsCmd := translateLineToJS(line)
		if jsCmd == "" {
			continue
		}

		jsLines = append(jsLines, jsCmd)
		lastJSCommand = jsCmd

		if defaultDelay > 0 && !strings.HasPrefix(jsCmd, "delay(") {
			jsLines = append(jsLines, fmt.Sprintf("delay(%d);", defaultDelay))
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading duckyscript: %w", err)
	}

	return strings.Join(jsLines, "\n"), nil
}

func translateLineToJS(line string) string {
	parts := strings.SplitN(line, " ", 2)
	cmd := strings.ToUpper(parts[0])
	arg := ""
	if len(parts) > 1 {
		arg = parts[1]
	}

	switch cmd {
	case "DELAY":
		ms, _ := strconv.Atoi(strings.TrimSpace(arg))
		return fmt.Sprintf("delay(%d);", ms)

	case "STRING":
		return fmt.Sprintf("type(%s);", strconv.Quote(arg))

	case "STRINGLN":
		return fmt.Sprintf("type(%s);\npress(\"ENTER\");", strconv.Quote(arg))

	case "LAYOUT":
		return fmt.Sprintf("layout(%s);", strconv.Quote(strings.TrimSpace(arg)))

	case "TYPING_SPEED":
		f := strings.Fields(arg)
		d, j := 10, 0
		if len(f) >= 1 {
			d, _ = strconv.Atoi(f[0])
		}
		if len(f) >= 2 {
			j, _ = strconv.Atoi(f[1])
		}
		return fmt.Sprintf("typingSpeed(%d, %d);", d, j)

	case "WAIT_LED":
		f := strings.Fields(arg)
		filter := "ANY"
		timeout := 5000
		if len(f) >= 1 {
			filter = f[0]
		}
		if len(f) >= 2 {
			timeout, _ = strconv.Atoi(f[1])
		}
		return fmt.Sprintf("waitLED(%s, %d);", strconv.Quote(filter), timeout)

	case "MOUSE_MOVE":
		f := strings.Fields(arg)
		x, y := 0, 0
		if len(f) >= 1 {
			x, _ = strconv.Atoi(f[0])
		}
		if len(f) >= 2 {
			y, _ = strconv.Atoi(f[1])
		}
		return fmt.Sprintf("mouseMove(%d, %d);", x, y)

	case "MOUSE_MOVE_ABS", "MOUSE_MOVETO":
		f := strings.Fields(arg)
		x, y := 0, 0
		if len(f) >= 1 {
			x, _ = strconv.Atoi(f[0])
		}
		if len(f) >= 2 {
			y, _ = strconv.Atoi(f[1])
		}
		return fmt.Sprintf("mouseMoveTo(%d, %d);", x, y)

	case "MOUSE_CLICK":
		btn := strings.TrimSpace(arg)
		if btn == "" {
			btn = "left"
		}
		return fmt.Sprintf("mouseClick(%s);", strconv.Quote(btn))

	default:
		// Check for key combinations like "GUI r", "CTRL-ALT DELETE", "ENTER", etc.
		normalized := strings.ReplaceAll(line, "-", " ")
		return fmt.Sprintf("press(%s);", strconv.Quote(normalized))
	}
}
