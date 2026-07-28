package api

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ScriptManager manages saving, loading, and deleting payload scripts.
type ScriptManager struct {
	mu        sync.RWMutex
	storageDir string
}

// NewScriptManager initializes the script manager with a target directory.
func NewScriptManager(storageDir string) (*ScriptManager, error) {
	if storageDir == "" {
		storageDir = "./payloads"
	}

	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create script storage directory: %w", err)
	}

	sm := &ScriptManager{
		storageDir: storageDir,
	}

	// Seed default scripts if directory is empty
	_ = sm.seedDefaultScripts()

	return sm, nil
}

// ListScripts returns all saved scripts from disk.
func (sm *ScriptManager) ListScripts() ([]Script, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	entries, err := os.ReadDir(sm.storageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read scripts directory: %w", err)
	}

	scripts := make([]Script, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".ducky") && !strings.HasSuffix(name, ".js") && !strings.HasSuffix(name, ".txt") {
			continue
		}

		filePath := filepath.Join(sm.storageDir, name)
		contentBytes, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		info, _ := entry.Info()
		modTime := time.Now()
		if info != nil {
			modTime = info.ModTime()
		}

		scriptType := "duckyscript"
		if strings.HasSuffix(name, ".js") {
			scriptType = "javascript"
		}

		scripts = append(scripts, Script{
			Name:        name,
			Type:        scriptType,
			Content:     string(contentBytes),
			Description: fmt.Sprintf("Saved %s payload", scriptType),
			UpdatedAt:   modTime,
		})
	}

	return scripts, nil
}

// SaveScript writes a script to disk.
func (sm *ScriptManager) SaveScript(s Script) error {
	if s.Name == "" {
		return errors.New("script name cannot be empty")
	}
	if strings.Contains(s.Name, "/") || strings.Contains(s.Name, "..") {
		return errors.New("invalid script name format")
	}
	if s.Content == "" {
		return errors.New("script content cannot be empty")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	filePath := filepath.Join(sm.storageDir, s.Name)
	err := os.WriteFile(filePath, []byte(s.Content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write script file: %w", err)
	}

	return nil
}

// DeleteScript removes a script from disk.
func (sm *ScriptManager) DeleteScript(name string) error {
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "..") {
		return errors.New("invalid script name")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	filePath := filepath.Join(sm.storageDir, name)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("script '%s' not found", name)
	}

	err := os.Remove(filePath)
	if err != nil {
		return fmt.Errorf("failed to delete script file: %w", err)
	}

	return nil
}

func (sm *ScriptManager) seedDefaultScripts() error {
	entries, err := os.ReadDir(sm.storageDir)
	if err == nil && len(entries) > 0 {
		return nil // Directory already populated
	}

	defaults := []Script{
		{
			Name: "windows_reverse_shell.ducky",
			Type: "duckyscript",
			Content: "REM Windows Powershell Launcher\n" +
				"DELAY 1000\n" +
				"GUI r\n" +
				"DELAY 500\n" +
				"STRING powershell -NoP -NonI -W Hidden -Exec Bypass -Command \"Write-Host Raspiducky Payload Executed!\"\n" +
				"ENTER\n",
		},
		{
			Name: "macos_terminal_opener.ducky",
			Type: "duckyscript",
			Content: "REM macOS Spotlight Launcher\n" +
				"DELAY 1000\n" +
				"GUI SPACE\n" +
				"DELAY 500\n" +
				"STRING Terminal\n" +
				"ENTER\n" +
				"DELAY 1000\n" +
				"STRING echo \"Raspiducky macOS Payload Active!\"\n" +
				"ENTER\n",
		},
		{
			Name: "mouse_jiggler.js",
			Type: "javascript",
			Content: "// Raspiducky JS Mouse Jiggler\n" +
				"console.log(\"Starting Mouse Jiggler loop...\");\n" +
				"for (let i = 0; i < 10; i++) {\n" +
				"    HID.moveMouse(10, 0);\n" +
				"    HID.delay(200);\n" +
				"    HID.moveMouse(0, 10);\n" +
				"    HID.delay(200);\n" +
				"    HID.moveMouse(-10, 0);\n" +
				"    HID.delay(200);\n" +
				"    HID.moveMouse(0, -10);\n" +
				"    HID.delay(200);\n" +
				"}\n" +
				"console.log(\"Jiggler finished!\");\n",
		},
	}

	for _, d := range defaults {
		_ = os.WriteFile(filepath.Join(sm.storageDir, d.Name), []byte(d.Content), 0644)
	}

	return nil
}
