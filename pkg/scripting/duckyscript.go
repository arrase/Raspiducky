// Package scripting provides the DuckyScript compiler and Goja JavaScript execution engine.
package scripting

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/arrase/Raspiducky/pkg/hid"
)

// Supported payload script types.
const (
	TypeDuckyScript = "duckyscript"
	TypeJavaScript  = "javascript"
)

// OpKind identifies the compiled DuckyScript instruction type.
type OpKind uint8

// Compiled DuckyScript instruction kinds.
const (
	OpDelay OpKind = iota + 1
	OpTypeString
	OpPress
	OpHold
	OpRelease
	OpLayout
	OpTypingSpeed
	OpMouseMove
	OpMouseMoveTo
	OpMouseClick
	OpWaitLED
)

// Instruction represents a validated, executable DuckyScript operation.
type Instruction struct {
	Kind      OpKind
	Line      int
	Text      string
	Int1      int
	Int2      int
	LEDMask   uint8
	Timeout   time.Duration
	PostDelay int
}

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

func normalizeLines(source string) []string {
	clean := strings.TrimPrefix(source, "\ufeff")
	clean = strings.ReplaceAll(clean, "\r\n", "\n")
	clean = strings.ReplaceAll(clean, "\r", "\n")
	return strings.Split(clean, "\n")
}

// CompileDuckyScript validates and compiles DuckyScript 1.0/2.0/3.0 source into executable instructions.
func CompileDuckyScript(duckySource, initialLayout string) ([]Instruction, error) {
	if initialLayout == "" {
		initialLayout = "US"
	}
	activeLayout, err := hid.GetLayout(initialLayout)
	if err != nil {
		activeLayout, _ = hid.GetLayout("US")
	}

	lines := normalizeLines(duckySource)
	var program []Instruction
	defaultDelay := 0
	var lastRepeatable *Instruction

	for idx := 0; idx < len(lines); idx++ {
		lineNum := idx + 1
		trimmed := strings.TrimLeft(lines[idx], " \t")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
			continue
		}

		cmd, arg, hasArg := splitCommandLine(trimmed)

		switch cmd {
		case "REM":
			continue

		case "REM_BLOCK":
			endIdx, ok := consumeUntil(lines, idx+1, "END_REM")
			if !ok {
				return nil, fmt.Errorf("line %d: unclosed REM_BLOCK (missing END_REM)", lineNum)
			}
			idx = endIdx
			continue

		case "ATTACKMODE":
			continue

		case "DEFAULT_DELAY", "DEFAULTDELAY":
			val, err := parseNonNegIntArg(lineNum, cmd, arg)
			if err != nil {
				return nil, err
			}
			defaultDelay = val
			continue

		case "DELAY":
			ms, err := parseNonNegIntArg(lineNum, cmd, arg)
			if err != nil {
				return nil, err
			}
			inst := Instruction{Kind: OpDelay, Line: lineNum, Int1: ms}
			program = append(program, inst)
			cp := inst
			lastRepeatable = &cp
			continue

		case "REPEAT":
			repeated, err := buildRepeatInstructions(lineNum, arg, lastRepeatable)
			if err != nil {
				return nil, err
			}
			program = append(program, repeated...)
			continue

		case "STRING", "STRINGLN":
			inst, nextIdx, err := parseStringInstruction(lines, idx, lineNum, cmd, arg, hasArg, defaultDelay)
			if err != nil {
				return nil, err
			}
			idx = nextIdx
			program = append(program, inst)
			cp := inst
			lastRepeatable = &cp
			continue
		}

		inst, repeatable, nextLayout, err := compileSingleLineCommand(lineNum, cmd, arg, trimmed, defaultDelay, activeLayout)
		if err != nil {
			return nil, err
		}
		if nextLayout != nil {
			activeLayout = nextLayout
		}
		program = append(program, inst)
		if repeatable {
			cp := inst
			lastRepeatable = &cp
		}
	}

	return program, nil
}

func splitCommandLine(line string) (string, string, bool) {
	idx := strings.IndexAny(line, " \t")
	if idx == -1 {
		return strings.ToUpper(line), "", false
	}
	return strings.ToUpper(line[:idx]), line[idx+1:], true
}

func consumeUntil(lines []string, startIdx int, endToken string) (int, bool) {
	for i := startIdx; i < len(lines); i++ {
		if strings.EqualFold(strings.TrimSpace(lines[i]), endToken) {
			return i, true
		}
	}
	return 0, false
}

func parseStringInstruction(lines []string, idx, lineNum int, cmd, arg string, hasArg bool, defaultDelay int) (Instruction, int, error) {
	if hasArg {
		text := arg
		if cmd == "STRINGLN" {
			text += "\n"
		}
		return Instruction{Kind: OpTypeString, Line: lineNum, Text: text, PostDelay: defaultDelay}, idx, nil
	}

	endToken := "END_STRING"
	if cmd == "STRINGLN" {
		endToken = "END_STRINGLN"
	}
	endIdx := -1
	for i := idx + 1; i < len(lines); i++ {
		tok := strings.ToUpper(strings.TrimSpace(lines[i]))
		if tok == endToken || (cmd == "STRINGLN" && tok == "END_STRING") {
			endIdx = i
			break
		}
	}
	if endIdx == -1 {
		return Instruction{}, idx, fmt.Errorf("line %d: unclosed %s block (missing %s)", lineNum, cmd, endToken)
	}

	block := strings.Join(lines[idx+1:endIdx], "\n")
	if cmd == "STRINGLN" {
		block += "\n"
	}
	return Instruction{Kind: OpTypeString, Line: lineNum, Text: block, PostDelay: defaultDelay}, endIdx, nil
}

func parseNonNegIntArg(lineNum int, cmd, arg string) (int, error) {
	fields := strings.Fields(arg)
	if len(fields) == 0 {
		return 0, fmt.Errorf("line %d: %s requires a numeric argument", lineNum, cmd)
	}
	if len(fields) > 1 {
		return 0, fmt.Errorf("line %d: invalid %s value %q", lineNum, cmd, strings.TrimSpace(arg))
	}
	val, err := strconv.Atoi(fields[0])
	if err != nil || val < 0 {
		return 0, fmt.Errorf("line %d: invalid %s value %q", lineNum, cmd, fields[0])
	}
	return val, nil
}

func buildRepeatInstructions(lineNum int, arg string, prev *Instruction) ([]Instruction, error) {
	if prev == nil {
		return nil, fmt.Errorf("line %d: REPEAT has no preceding command to repeat", lineNum)
	}
	fields := strings.Fields(arg)
	if len(fields) == 0 {
		return nil, fmt.Errorf("line %d: REPEAT requires a count argument", lineNum)
	}
	if len(fields) > 1 {
		return nil, fmt.Errorf("line %d: invalid REPEAT count %q", lineNum, strings.TrimSpace(arg))
	}
	count, err := strconv.Atoi(fields[0])
	if err != nil || count <= 0 {
		return nil, fmt.Errorf("line %d: invalid REPEAT count %q", lineNum, fields[0])
	}

	out := make([]Instruction, count)
	for i := range count {
		inst := *prev
		inst.Line = lineNum
		out[i] = inst
	}
	return out, nil
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

func compileMouseInstruction(lineNum int, cmd, arg string, defaultDelay int) (Instruction, bool, error) {
	switch cmd {
	case "MOUSE_MOVE":
		x, y, err := parseCoordinates(lineNum, cmd, arg)
		if err != nil {
			return Instruction{}, false, err
		}
		return Instruction{Kind: OpMouseMove, Line: lineNum, Int1: x, Int2: y, PostDelay: defaultDelay}, true, nil

	case "MOUSE_MOVE_ABS", "MOUSE_MOVETO":
		x, y, err := parseCoordinates(lineNum, cmd, arg)
		if err != nil {
			return Instruction{}, false, err
		}
		return Instruction{Kind: OpMouseMoveTo, Line: lineNum, Int1: x, Int2: y, PostDelay: defaultDelay}, true, nil

	case "MOUSE_CLICK":
		btn := strings.ToLower(strings.TrimSpace(arg))
		switch btn {
		case "", "left", "1", "b1", "button1", "right", "2", "b2", "button2", "middle", "3", "b3", "button3":
		default:
			return Instruction{}, false, fmt.Errorf("line %d: invalid MOUSE_CLICK button %q", lineNum, arg)
		}
		if btn == "" {
			btn = "left"
		}
		return Instruction{Kind: OpMouseClick, Line: lineNum, Text: btn, PostDelay: defaultDelay}, true, nil

	default:
		return Instruction{}, false, nil
	}
}

func compileTypingSpeedInstruction(lineNum int, cmd, arg string) (Instruction, error) {
	f := strings.Fields(arg)
	if len(f) == 0 || len(f) > 2 {
		return Instruction{}, fmt.Errorf("line %d: %s requires 1 or 2 integer arguments", lineNum, cmd)
	}
	d, err := strconv.Atoi(f[0])
	if err != nil || d < 0 {
		return Instruction{}, fmt.Errorf("line %d: invalid %s delay %q", lineNum, cmd, f[0])
	}
	j := 0
	if len(f) == 2 {
		j, err = strconv.Atoi(f[1])
		if err != nil || j < 0 {
			return Instruction{}, fmt.Errorf("line %d: invalid %s jitter %q", lineNum, cmd, f[1])
		}
	}
	return Instruction{Kind: OpTypingSpeed, Line: lineNum, Int1: d, Int2: j}, nil
}

func compileWaitLEDInstruction(lineNum int, arg string, defaultDelay int) (Instruction, error) {
	f := strings.Fields(arg)
	if len(f) > 2 {
		return Instruction{}, fmt.Errorf("line %d: invalid WAIT_LED arguments %q", lineNum, strings.TrimSpace(arg))
	}
	filter := "ANY"
	timeoutMs := 5000
	if len(f) >= 1 {
		filter = f[0]
	}
	mask, err := hid.ParseLEDMask(filter)
	if err != nil {
		return Instruction{}, fmt.Errorf("line %d: %w", lineNum, err)
	}
	if len(f) == 2 {
		timeoutMs, err = strconv.Atoi(f[1])
		if err != nil || timeoutMs <= 0 {
			return Instruction{}, fmt.Errorf("line %d: invalid WAIT_LED timeout %q", lineNum, f[1])
		}
	}
	return Instruction{
		Kind:      OpWaitLED,
		Line:      lineNum,
		LEDMask:   mask,
		Timeout:   time.Duration(timeoutMs) * time.Millisecond,
		PostDelay: defaultDelay,
	}, nil
}

func compileSingleLineCommand(lineNum int, cmd, arg, fullLine string, defaultDelay int, layout *hid.Layout) (Instruction, bool, *hid.Layout, error) {
	if inst, ok, err := compileMouseInstruction(lineNum, cmd, arg, defaultDelay); ok || err != nil {
		return inst, true, nil, err
	}

	switch cmd {
	case "LAYOUT":
		trimmed := strings.TrimSpace(arg)
		if trimmed == "" {
			return Instruction{}, false, nil, fmt.Errorf("line %d: LAYOUT requires a layout argument", lineNum)
		}
		nextLayout, err := hid.GetLayout(trimmed)
		if err != nil {
			return Instruction{}, false, nil, fmt.Errorf("line %d: %w", lineNum, err)
		}
		return Instruction{Kind: OpLayout, Line: lineNum, Text: nextLayout.Name}, false, nextLayout, nil

	case "TYPING_SPEED", "STRING_DELAY", "STRINGDELAY":
		inst, err := compileTypingSpeedInstruction(lineNum, cmd, arg)
		return inst, false, nil, err

	case "WAIT_LED":
		inst, err := compileWaitLEDInstruction(lineNum, arg, defaultDelay)
		return inst, true, nil, err

	case "HOLD", "RELEASE":
		tokens := hid.SplitComboTokens(arg)
		if len(tokens) == 0 {
			return Instruction{}, false, nil, fmt.Errorf("line %d: %s requires a key argument", lineNum, cmd)
		}
		joined := strings.Join(tokens, " ")
		if err := hid.ValidateCombo(layout, joined); err != nil {
			return Instruction{}, false, nil, fmt.Errorf("line %d: invalid %s key: %w", lineNum, cmd, err)
		}
		kind := OpHold
		if cmd == "RELEASE" {
			kind = OpRelease
		}
		return Instruction{Kind: kind, Line: lineNum, Text: joined, PostDelay: defaultDelay}, true, nil, nil

	default:
		tokens := hid.SplitComboTokens(fullLine)
		joined := strings.Join(tokens, " ")
		if err := hid.ValidateCombo(layout, joined); err != nil {
			return Instruction{}, false, nil, fmt.Errorf("line %d: unknown command or key in %q: %w", lineNum, strings.TrimSpace(fullLine), err)
		}
		return Instruction{Kind: OpPress, Line: lineNum, Text: joined, PostDelay: defaultDelay}, true, nil, nil
	}
}
