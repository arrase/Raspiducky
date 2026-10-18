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
STRING a
repeat 3
`
	js, err := ParseDuckyScript(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedLines := []string{
		`// initial comment`,
		`// Default delay set to 50 ms`,
		`type("a");`,
		`delay(50);`,
		`for (let _i = 0; _i < 3; _i++) { type("a"); delay(50); }`,
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
			name:    "repeat without prior command",
			src:     "REM comment\nREPEAT 2",
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
