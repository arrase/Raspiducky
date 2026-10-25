package scripting

import (
	"strings"
	"testing"
)

func TestParseDuckyScript_StringTrailingAndSingleSpaces(t *testing.T) {
	src := "STRING hello \nSTRING  \nSTRINGLN world \n"
	js, err := ParseDuckyScript(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedLines := []string{
		`type("hello ");`,
		`type(" ");`,
		`type("world ");`,
		`press("ENTER");`,
	}
	expected := strings.Join(expectedLines, "\n")
	if js != expected {
		t.Errorf("got:\n%s\nwant:\n%s", js, expected)
	}
}

func TestParseDuckyScript_CaseInsensitiveAndRepeatDefaultDelay(t *testing.T) {
	src := `rem initial comment
default_delay 50
LAYOUT ES
TYPING_SPEED 15
STRING a
repeat 3
STRINGLN b
DELAY	200
REPEAT 2
STRING	tabbed
`
	js, err := ParseDuckyScript(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedLines := []string{
		`// initial comment`,
		`// Default delay set to 50 ms`,
		`layout("ES");`,
		`typingSpeed(15, 0);`,
		`type("a");`,
		`delay(50);`,
		`for (let _i = 0; _i < 3; _i++) { type("a"); delay(50); }`,
		`type("b");`,
		`press("ENTER");`,
		`delay(50);`,
		`delay(200);`,
		`for (let _i = 0; _i < 2; _i++) { delay(200); }`,
		`type("tabbed");`,
		`delay(50);`,
	}
	expected := strings.Join(expectedLines, "\n")
	if js != expected {
		t.Errorf("got:\n%s\nwant:\n%s", js, expected)
	}
}

func TestParseDuckyScript_Errors(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantSub string
	}{
		{
			name:    "default_delay missing arg",
			src:     "DEFAULT_DELAY",
			wantSub: "line 1",
		},
		{
			name:    "defaultdelay negative",
			src:     "REM ok\ndefaultdelay -10",
			wantSub: "line 2",
		},
		{
			name:    "default_delay non-int",
			src:     "DEFAULT_DELAY abc",
			wantSub: "line 1",
		},
		{
			name:    "default_delay extra token",
			src:     "DEFAULT_DELAY 100 ENTER",
			wantSub: "line 1",
		},
		{
			name:    "delay missing arg",
			src:     "DELAY",
			wantSub: "line 1",
		},
		{
			name:    "delay negative",
			src:     "DELAY -50",
			wantSub: "line 1",
		},
		{
			name:    "delay non-int",
			src:     "DELAY abc",
			wantSub: "line 1",
		},
		{
			name:    "delay concatenated command",
			src:     "DELAY 500ENTER",
			wantSub: "line 1",
		},
		{
			name:    "delay extra command on same line",
			src:     "DELAY 500 ENTER",
			wantSub: "line 1",
		},
		{
			name:    "repeat without prior command",
			src:     "REM comment\nREPEAT 2",
			wantSub: "line 2",
		},
		{
			name:    "repeat after layout directive without action command",
			src:     "LAYOUT ES\nREPEAT 2",
			wantSub: "line 2",
		},
		{
			name:    "repeat missing count",
			src:     "ENTER\nREPEAT",
			wantSub: "line 2",
		},
		{
			name:    "repeat zero count",
			src:     "ENTER\nrepeat 0",
			wantSub: "line 2",
		},
		{
			name:    "repeat negative count",
			src:     "ENTER\nREPEAT -3",
			wantSub: "line 2",
		},
		{
			name:    "repeat extra token",
			src:     "ENTER\nREPEAT 2 ENTER",
			wantSub: "line 2",
		},
		{
			name:    "mouse_move invalid x",
			src:     "MOUSE_MOVE abc 10",
			wantSub: "line 1",
		},
		{
			name:    "mouse_moveto missing y",
			src:     "MOUSE_MOVETO 100",
			wantSub: "line 1",
		},
		{
			name:    "wait_led invalid timeout",
			src:     "WAIT_LED CAPS abc",
			wantSub: "line 1",
		},
		{
			name:    "typing_speed invalid arg",
			src:     "TYPING_SPEED abc",
			wantSub: "line 1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseDuckyScript(tc.src)
			if err == nil {
				t.Fatalf("expected error for %q, got nil", tc.src)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("expected error %q to contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}

func TestParseDuckyScript_KeyCombosAndMinus(t *testing.T) {
	src := strings.Join([]string{
		"-",
		"CTRL -",
		"ALT--",
		"CTRL-ALT--",
		"CTRL-ALT-DELETE",
		"CTRL-S",
		"GUI r",
	}, "\n")

	js, err := ParseDuckyScript(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedLines := []string{
		`press("-");`,
		`press("CTRL -");`,
		`press("ALT -");`,
		`press("CTRL ALT -");`,
		`press("CTRL ALT DELETE");`,
		`press("CTRL S");`,
		`press("GUI r");`,
	}
	expected := strings.Join(expectedLines, "\n")
	if js != expected {
		t.Errorf("got:\n%s\nwant:\n%s", js, expected)
	}
}

func TestParseDuckyScript_Commands(t *testing.T) {
	src := strings.Join([]string{
		"LAYOUT ES",
		"TYPING_SPEED 15 5",
		"DELAY 100",
		"MOUSE_MOVE 10 -20",
		"MOUSE_MOVETO 1000 2000",
		"MOUSE_CLICK",
		"MOUSE_CLICK right",
		"WAIT_LED CAPS 2000",
	}, "\n")

	js, err := ParseDuckyScript(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedLines := []string{
		`layout("ES");`,
		`typingSpeed(15, 5);`,
		`delay(100);`,
		`mouseMove(10, -20);`,
		`mouseMoveTo(1000, 2000);`,
		`mouseClick("left");`,
		`mouseClick("right");`,
		`waitLED("CAPS", 2000);`,
	}
	expected := strings.Join(expectedLines, "\n")
	if js != expected {
		t.Errorf("got:\n%s\nwant:\n%s", js, expected)
	}
}
