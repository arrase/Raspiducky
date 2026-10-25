package scripting

import (
	"bufio"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	TypeDuckyScript = "duckyscript"
	TypeJavaScript  = "javascript"
)

// DetectScriptType returns TypeJavaScript or TypeDuckyScript from an explicit type string or filename extension.
func DetectScriptType(scriptType, fileName string) string {
	t := strings.ToLower(strings.TrimSpace(scriptType))
	if t == TypeJavaScript || t == "js" {
		return TypeJavaScript
	}
	if t == TypeDuckyScript || t == "ducky" || t == "txt" {
		return TypeDuckyScript
	}
	if strings.EqualFold(filepath.Ext(fileName), ".js") {
		return TypeJavaScript
	}
	return TypeDuckyScript
}

// ParseDuckyScript parses Rubber Ducky script syntax into equivalent JavaScript code.
func ParseDuckyScript(duckySource string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(duckySource))
	var jsLines []string

	defaultDelay := 0
	var lastJSCommand string
	lastAppliesDelay := false
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimLeft(strings.TrimRight(scanner.Text(), "\r\n"), " \t")
		if line == "" {
			continue
		}

		cmd, arg := splitCommandLine(line)

		switch cmd {
		case "REM":
			jsLines = append(jsLines, fmt.Sprintf("// %s", strings.TrimSpace(arg)))
			continue

		case "DEFAULT_DELAY", "DEFAULTDELAY":
			delayVal, err := parseDelayArg(lineNum, cmd, arg)
			if err != nil {
				return "", err
			}
			defaultDelay = delayVal
			jsLines = append(jsLines, fmt.Sprintf("// Default delay set to %d ms", delayVal))
			continue

		case "DELAY":
			ms, err := parseDelayArg(lineNum, cmd, arg)
			if err != nil {
				return "", err
			}
			jsCmd := fmt.Sprintf("delay(%d);", ms)
			jsLines = append(jsLines, jsCmd)
			lastJSCommand = jsCmd
			lastAppliesDelay = false
			continue

		case "REPEAT":
			repeatJS, err := parseRepeatCommand(lineNum, arg, lastJSCommand, defaultDelay, lastAppliesDelay)
			if err != nil {
				return "", err
			}
			jsLines = append(jsLines, repeatJS)
			continue
		}

		jsCmd, appliesDelay, err := translateCommandToJS(lineNum, cmd, arg, line)
		if err != nil {
			return "", err
		}
		jsLines = append(jsLines, jsCmd)
		if appliesDelay {
			if defaultDelay > 0 {
				jsLines = append(jsLines, fmt.Sprintf("delay(%d);", defaultDelay))
			}
			lastJSCommand = jsCmd
			lastAppliesDelay = true
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading duckyscript: %w", err)
	}

	return strings.Join(jsLines, "\n"), nil
}

func splitCommandLine(line string) (string, string) {
	idx := strings.IndexAny(line, " \t")
	if idx == -1 {
		return strings.ToUpper(line), ""
	}
	return strings.ToUpper(line[:idx]), line[idx+1:]
}

func parseDelayArg(lineNum int, cmd, arg string) (int, error) {
	fields := strings.Fields(arg)
	if len(fields) == 0 {
		return 0, fmt.Errorf("line %d: %s requires a delay argument", lineNum, cmd)
	}
	if len(fields) > 1 {
		return 0, fmt.Errorf("line %d: invalid %s value %q", lineNum, cmd, strings.TrimSpace(arg))
	}
	delayVal, err := strconv.Atoi(fields[0])
	if err != nil || delayVal < 0 {
		return 0, fmt.Errorf("line %d: invalid %s value %q", lineNum, cmd, fields[0])
	}
	return delayVal, nil
}

func parseRepeatCommand(lineNum int, arg, lastJSCommand string, defaultDelay int, appliesDelay bool) (string, error) {
	if lastJSCommand == "" {
		return "", fmt.Errorf("line %d: REPEAT has no preceding command to repeat", lineNum)
	}
	fields := strings.Fields(arg)
	if len(fields) == 0 {
		return "", fmt.Errorf("line %d: REPEAT requires a count argument", lineNum)
	}
	if len(fields) > 1 {
		return "", fmt.Errorf("line %d: invalid REPEAT count %q", lineNum, strings.TrimSpace(arg))
	}
	count, err := strconv.Atoi(fields[0])
	if err != nil || count <= 0 {
		return "", fmt.Errorf("line %d: invalid REPEAT count %q", lineNum, fields[0])
	}
	body := lastJSCommand
	if appliesDelay && defaultDelay > 0 {
		body = fmt.Sprintf("%s delay(%d);", lastJSCommand, defaultDelay)
	}
	return fmt.Sprintf("for (let _i = 0; _i < %d; _i++) { %s }", count, body), nil
}

func parseCoordinates(lineNum int, cmd, arg string) (int, int, error) {
	f := strings.Fields(arg)
	if len(f) != 2 {
		return 0, 0, fmt.Errorf("line %d: %s requires X and Y integer arguments", lineNum, cmd)
	}
	x, err := strconv.Atoi(f[0])
	if err != nil {
		return 0, 0, fmt.Errorf("line %d: invalid %s X coordinate %q", lineNum, cmd, f[0])
	}
	y, err := strconv.Atoi(f[1])
	if err != nil {
		return 0, 0, fmt.Errorf("line %d: invalid %s Y coordinate %q", lineNum, cmd, f[1])
	}
	return x, y, nil
}

func translateMouseCommand(lineNum int, cmd, arg string) (string, bool, error) {
	switch cmd {
	case "MOUSE_MOVE":
		x, y, err := parseCoordinates(lineNum, cmd, arg)
		if err != nil {
			return "", false, err
		}
		return fmt.Sprintf("mouseMove(%d, %d);", x, y), true, nil
	case "MOUSE_MOVE_ABS", "MOUSE_MOVETO":
		x, y, err := parseCoordinates(lineNum, cmd, arg)
		if err != nil {
			return "", false, err
		}
		return fmt.Sprintf("mouseMoveTo(%d, %d);", x, y), true, nil
	case "MOUSE_CLICK":
		btn := strings.TrimSpace(arg)
		if btn == "" {
			btn = "left"
		}
		return fmt.Sprintf("mouseClick(%s);", strconv.Quote(btn)), true, nil
	default:
		return "", false, nil
	}
}

func translateCommandToJS(lineNum int, cmd, arg, line string) (string, bool, error) {
	if js, ok, err := translateMouseCommand(lineNum, cmd, arg); ok || err != nil {
		return js, true, err
	}

	switch cmd {
	case "STRING":
		return fmt.Sprintf("type(%s);", strconv.Quote(arg)), true, nil

	case "STRINGLN":
		return fmt.Sprintf("type(%s);\npress(\"ENTER\");", strconv.Quote(arg)), true, nil

	case "LAYOUT":
		trimmed := strings.TrimSpace(arg)
		if trimmed == "" {
			return "", false, fmt.Errorf("line %d: LAYOUT requires a layout argument", lineNum)
		}
		return fmt.Sprintf("layout(%s);", strconv.Quote(trimmed)), false, nil

	case "TYPING_SPEED":
		f := strings.Fields(arg)
		if len(f) == 0 || len(f) > 2 {
			return "", false, fmt.Errorf("line %d: TYPING_SPEED requires 1 or 2 integer arguments", lineNum)
		}
		d, err := strconv.Atoi(f[0])
		if err != nil || d < 0 {
			return "", false, fmt.Errorf("line %d: invalid TYPING_SPEED delay %q", lineNum, f[0])
		}
		j := 0
		if len(f) == 2 {
			j, err = strconv.Atoi(f[1])
			if err != nil || j < 0 {
				return "", false, fmt.Errorf("line %d: invalid TYPING_SPEED jitter %q", lineNum, f[1])
			}
		}
		return fmt.Sprintf("typingSpeed(%d, %d);", d, j), false, nil

	case "WAIT_LED":
		f := strings.Fields(arg)
		if len(f) > 2 {
			return "", false, fmt.Errorf("line %d: invalid WAIT_LED arguments %q", lineNum, strings.TrimSpace(arg))
		}
		filter := "ANY"
		timeout := 5000
		if len(f) >= 1 {
			filter = f[0]
		}
		if len(f) == 2 {
			var err error
			timeout, err = strconv.Atoi(f[1])
			if err != nil || timeout <= 0 {
				return "", false, fmt.Errorf("line %d: invalid WAIT_LED timeout %q", lineNum, f[1])
			}
		}
		return fmt.Sprintf("waitLED(%s, %d);", strconv.Quote(filter), timeout), true, nil

	default:
		return parseComboOrSingleKey(line), true, nil
	}
}

func parseComboOrSingleKey(line string) string {
	var tokens []string
	for _, field := range strings.Fields(line) {
		if field == "-" {
			tokens = append(tokens, "-")
			continue
		}
		hasTrailingMinus := strings.HasSuffix(field, "--")
		target := field
		if hasTrailingMinus {
			target = field[:len(field)-1]
		}
		for _, part := range strings.Split(target, "-") {
			if part != "" {
				tokens = append(tokens, part)
			}
		}
		if hasTrailingMinus {
			tokens = append(tokens, "-")
		}
	}
	return fmt.Sprintf("press(%s);", strconv.Quote(strings.Join(tokens, " ")))
}
