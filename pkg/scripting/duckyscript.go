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
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimLeft(strings.TrimRight(scanner.Text(), "\r\n"), " \t")
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		cmd := strings.ToUpper(parts[0])
		arg := ""
		if len(parts) > 1 {
			arg = parts[1]
		}

		switch cmd {
		case "REM":
			jsLines = append(jsLines, fmt.Sprintf("// %s", strings.TrimSpace(arg)))
			continue

		case "DEFAULT_DELAY", "DEFAULTDELAY":
			fields := strings.Fields(arg)
			if len(fields) == 0 {
				return "", fmt.Errorf("line %d: %s requires a delay argument", lineNum, cmd)
			}
			delayVal, err := strconv.Atoi(fields[0])
			if err != nil || delayVal < 0 {
				return "", fmt.Errorf("line %d: invalid %s value %q", lineNum, cmd, fields[0])
			}
			defaultDelay = delayVal
			jsLines = append(jsLines, fmt.Sprintf("// Default delay set to %d ms", delayVal))
			continue

		case "REPEAT":
			if lastJSCommand == "" {
				return "", fmt.Errorf("line %d: REPEAT has no preceding command to repeat", lineNum)
			}
			fields := strings.Fields(arg)
			if len(fields) == 0 {
				return "", fmt.Errorf("line %d: REPEAT requires a count argument", lineNum)
			}
			count, err := strconv.Atoi(fields[0])
			if err != nil || count <= 0 {
				return "", fmt.Errorf("line %d: invalid REPEAT count %q", lineNum, fields[0])
			}
			body := lastJSCommand
			if defaultDelay > 0 {
				body = fmt.Sprintf("%s delay(%d);", lastJSCommand, defaultDelay)
			}
			jsLines = append(jsLines, fmt.Sprintf("for (let _i = 0; _i < %d; _i++) { %s }", count, body))
			continue
		}

		jsCmd := translateCommandToJS(cmd, arg, line)
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

func translateCommandToJS(cmd, arg, line string) string {
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
		return parseComboOrSingleKey(line)
	}
}

func parseComboOrSingleKey(line string) string {
	var tokens []string
	for _, field := range strings.Fields(line) {
		if field == "-" {
			tokens = append(tokens, "-")
			continue
		}
		for _, part := range strings.Split(field, "-") {
			if part != "" {
				tokens = append(tokens, part)
			}
		}
	}
	return fmt.Sprintf("press(%s);", strconv.Quote(strings.Join(tokens, " ")))
}
