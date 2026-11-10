package scripting

import (
	"strings"
	"testing"
	"time"

	"github.com/arrase/Raspiducky/pkg/hid"
)

func TestCompileDuckyScript_StringTrailingSpacesAndMultilineBlocks(t *testing.T) {
	src := "\ufeffSTRING hello \r\nSTRING  \r\nSTRINGLN world \r\nSTRING\nline 1\n\nline 3\nEND_STRING\nSTRINGLN\nblock ln\nEND_STRINGLN\n"
	prog, err := CompileDuckyScript(src, "US")
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}

	wantTexts := []string{
		"hello ",
		" ",
		"world \n",
		"line 1\n\nline 3",
		"block ln\n",
	}
	if len(prog) != len(wantTexts) {
		t.Fatalf("expected %d instructions, got %d: %+v", len(wantTexts), len(prog), prog)
	}
	for i, want := range wantTexts {
		if prog[i].Kind != OpTypeString || prog[i].Text != want {
			t.Errorf("inst[%d] = (%v, %q), want (OpTypeString, %q)", i, prog[i].Kind, prog[i].Text, want)
		}
	}
}

func TestCompileDuckyScript_CommentsRepeatAndSingleDefaultDelayOnStringln(t *testing.T) {
	src := `rem initial comment
# shell style comment
// c style comment
REM_BLOCK
ignored invalid command inside block
END_REM
ATTACKMODE HID STORAGE
default_delay 50
LAYOUT ES
STRING_DELAY 15 3
STRING a
repeat 2
STRINGLN b
DELAY	200
REPEAT 2
`
	prog, err := CompileDuckyScript(src, "US")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected sequence:
	// 0: OpLayout("ES")
	// 1: OpTypingSpeed(15, 3)
	// 2: OpTypeString("a", PostDelay=50)
	// 3,4: repeated OpTypeString("a", PostDelay=50)
	// 5: OpTypeString("b\n", PostDelay=50) -- single PostDelay, NOT doubled!
	// 6: OpDelay(200, PostDelay=0)
	// 7,8: repeated OpDelay(200, PostDelay=0)
	if len(prog) != 9 {
		t.Fatalf("expected 9 instructions, got %d: %+v", len(prog), prog)
	}
	if prog[0].Kind != OpLayout || prog[0].Text != "ES" {
		t.Errorf("unexpected layout inst: %+v", prog[0])
	}
	if prog[1].Kind != OpTypingSpeed || prog[1].Int1 != 15 || prog[1].Int2 != 3 {
		t.Errorf("unexpected typing speed inst: %+v", prog[1])
	}
	for _, i := range []int{2, 3, 4} {
		if prog[i].Kind != OpTypeString || prog[i].Text != "a" || prog[i].PostDelay != 50 {
			t.Errorf("unexpected STRING/REPEAT inst[%d]: %+v", i, prog[i])
		}
	}
	if prog[5].Kind != OpTypeString || prog[5].Text != "b\n" || prog[5].PostDelay != 50 {
		t.Errorf("unexpected STRINGLN inst[5]: %+v", prog[5])
	}
	for _, i := range []int{6, 7, 8} {
		if prog[i].Kind != OpDelay || prog[i].Int1 != 200 || prog[i].PostDelay != 0 {
			t.Errorf("unexpected DELAY/REPEAT inst[%d]: %+v", i, prog[i])
		}
	}
}

func TestCompileDuckyScript_Errors(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantSub string
	}{
		{"default_delay missing arg", "DEFAULT_DELAY", "line 1"},
		{"defaultdelay negative", "REM ok\ndefaultdelay -10", "line 2"},
		{"default_delay non-int", "DEFAULT_DELAY abc", "line 1"},
		{"default_delay extra token", "DEFAULT_DELAY 100 ENTER", "line 1"},
		{"delay missing arg", "DELAY", "line 1"},
		{"delay negative", "DELAY -50", "line 1"},
		{"delay non-int", "DELAY abc", "line 1"},
		{"delay concatenated command", "DELAY 500ENTER", "line 1"},
		{"delay extra command on same line", "DELAY 500 ENTER", "line 1"},
		{"repeat without prior command", "REM comment\nREPEAT 2", "line 2"},
		{"repeat after layout directive without action command", "LAYOUT ES\nREPEAT 2", "line 2"},
		{"repeat missing count", "ENTER\nREPEAT", "line 2"},
		{"repeat zero count", "ENTER\nrepeat 0", "line 2"},
		{"repeat negative count", "ENTER\nREPEAT -3", "line 2"},
		{"repeat exceeds max count", "ENTER\nREPEAT 10001", "line 2"},
		{"repeat extra token", "ENTER\nREPEAT 2 ENTER", "line 2"},
		{"mouse_move invalid x", "MOUSE_MOVE abc 10", "line 1"},
		{"mouse_moveto missing y", "MOUSE_MOVETO 100", "line 1"},
		{"mouse_click invalid button", "MOUSE_CLICK side", "line 1"},
		{"wait_led invalid mask", "WAIT_LED TYPO 1000", "line 1"},
		{"wait_led invalid timeout", "WAIT_LED CAPS abc", "line 1"},
		{"typing_speed invalid arg", "TYPING_SPEED abc", "line 1"},
		{"layout missing arg", "LAYOUT", "line 1"},
		{"layout unknown", "LAYOUT INVALID_LANG", "line 1"},
		{"hold missing arg", "HOLD", "line 1"},
		{"hold invalid key", "HOLD NOTAKEY", "line 1"},
		{"release missing arg", "RELEASE", "line 1"},
		{"unknown command typo caught at compile time", "REM ok\nSTRNG hello", "line 2"},
		{"unclosed multiline string", "STRING\nhello", "line 1"},
		{"unclosed rem_block", "REM_BLOCK\ncomment", "line 1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CompileDuckyScript(tc.src, "US")
			if err == nil {
				t.Fatalf("expected error for %q, got nil", tc.src)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("expected error %q to contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}

func TestCompileDuckyScript_KeyCombosAndCommands(t *testing.T) {
	src := strings.Join([]string{
		"-",
		"CTRL -",
		"ALT--",
		"CTRL-ALT--",
		"CTRL-ALT-DELETE",
		"CTRL+S",
		"GUI r",
		"LAYOUT ES",
		"ñ",
		"MOUSE_MOVE 10 -20",
		"MOUSE_MOVETO 1000 2000",
		"MOUSE_CLICK",
		"MOUSE_CLICK right",
		"WAIT_LED CAPS 2000",
		"HOLD SHIFT",
		"RELEASE SHIFT",
	}, "\n")

	prog, err := CompileDuckyScript(src, "US")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantPress := []string{
		"-",
		"CTRL -",
		"ALT -",
		"CTRL ALT -",
		"CTRL ALT DELETE",
		"CTRL S",
		"GUI r",
	}
	for i, want := range wantPress {
		if prog[i].Kind != OpPress || prog[i].Text != want {
			t.Errorf("prog[%d] = (%v, %q), want (OpPress, %q)", i, prog[i].Kind, prog[i].Text, want)
		}
	}
	if prog[8].Kind != OpPress || prog[8].Text != "ñ" {
		t.Errorf("expected ES layout key 'ñ' to compile cleanly after LAYOUT ES, got %+v", prog[8])
	}
	if prog[13].Kind != OpWaitLED || prog[13].LEDMask != hid.LEDCapsLock || prog[13].Timeout != 2*time.Second {
		t.Errorf("unexpected WAIT_LED instruction: %+v", prog[13])
	}
}
