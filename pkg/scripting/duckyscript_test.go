package scripting

import (
	"strings"
	"testing"
)

func TestParseDuckyScript(t *testing.T) {
	ducky := `
REM Test script
DEFAULT_DELAY 100
DELAY 500
STRING Hello World
STRINGLN Test Line
GUI r
REPEAT 2
`

	js, err := ParseDuckyScript(ducky)
	if err != nil {
		t.Fatalf("ParseDuckyScript failed: %v", err)
	}

	expectedParts := []string{
		"// Test script",
		"delay(500);",
		"type(\"Hello World\");",
		"type(\"Test Line\");",
		"press(\"ENTER\");",
		"press(\"GUI r\");",
	}

	for _, part := range expectedParts {
		if !strings.Contains(js, part) {
			t.Errorf("Expected JS to contain %q, got:\n%s", part, js)
		}
	}
}
