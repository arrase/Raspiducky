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
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if comment, ok := parseComment(line); ok {
			jsLines = append(jsLines, comment)
			continue
		}

		if delay, comment, ok := parseDefaultDelay(line); ok {
			defaultDelay = delay
			jsLines = append(jsLines, comment)
			continue
		}

		if repeatLines, ok := parseRepeat(line, lastJSCommand, defaultDelay); ok {
			jsLines = append(jsLines, repeatLines...)
			continue
		}

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

func parseComment(line string) (string, bool) {
	if line == "REM" || strings.HasPrefix(line, "REM ") {
		comment := strings.TrimSpace(strings.TrimPrefix(line, "REM"))
		return fmt.Sprintf("// %s", comment), true
	}
	return "", false
}

func parseDefaultDelay(line string) (int, string, bool) {
	if strings.HasPrefix(line, "DEFAULT_DELAY") || strings.HasPrefix(line, "DEFAULTDELAY") {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			if delayVal, err := strconv.Atoi(parts[1]); err == nil {
				return delayVal, fmt.Sprintf("// Default delay set to %d ms", delayVal), true
			}
		}
	}
	return 0, "", false
}

func parseRepeat(line, lastJSCommand string, defaultDelay int) ([]string, bool) {
	if !strings.HasPrefix(line, "REPEAT") || lastJSCommand == "" {
		return nil, false
	}
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return nil, false
	}
	count, err := strconv.Atoi(parts[1])
	if err != nil || count <= 0 {
		return nil, false
	}
	res := []string{fmt.Sprintf("for (let _i = 0; _i < %d; _i++) { %s }", count, lastJSCommand)}
	if defaultDelay > 0 {
		res = append(res, fmt.Sprintf("delay(%d);", defaultDelay))
	}
	return res, true
}

func parseCoordinates(arg string) (int, int) {
	f := strings.Fields(arg)
	x, y := 0, 0
	if len(f) >= 1 {
		x, _ = strconv.Atoi(f[0])
	}
	if len(f) >= 2 {
		y, _ = strconv.Atoi(f[1])
	}
	return x, y
}

func translateMouseCommand(cmd, arg string) (string, bool) {
	switch cmd {
	case "MOUSE_MOVE":
		x, y := parseCoordinates(arg)
		return fmt.Sprintf("mouseMove(%d, %d);", x, y), true
	case "MOUSE_MOVE_ABS", "MOUSE_MOVETO":
		x, y := parseCoordinates(arg)
		return fmt.Sprintf("mouseMoveTo(%d, %d);", x, y), true
	case "MOUSE_CLICK":
		btn := strings.TrimSpace(arg)
		if btn == "" {
			btn = "left"
		}
		return fmt.Sprintf("mouseClick(%s);", strconv.Quote(btn)), true
	default:
		return "", false
	}
}

func translateLineToJS(line string) string {
	parts := strings.SplitN(line, " ", 2)
	cmd := strings.ToUpper(parts[0])
	arg := ""
	if len(parts) > 1 {
		arg = parts[1]
	}

	if js, ok := translateMouseCommand(cmd, arg); ok {
		return js
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
		d, j := parseCoordinates(arg)
		if len(strings.Fields(arg)) == 0 {
			d = 10
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

	default:
		// Check for key combinations like "GUI r", "CTRL-ALT DELETE", "ENTER", etc.
		normalized := strings.ReplaceAll(line, "-", " ")
		return fmt.Sprintf("press(%s);", strconv.Quote(normalized))
	}
}
