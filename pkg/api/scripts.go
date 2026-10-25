package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/arrase/Raspiducky/pkg/scripting"
)

const (
	defaultDelay1000 = "DELAY 1000\n"
	defaultEnter     = "ENTER\n"
	defaultJSDelay   = "    delay(200);\n"
	duckyExt         = ".ducky"
	scriptsMetaFile  = "scripts_meta.json"
)

var (
	ErrInvalidScriptName = errors.New("invalid script name")
	ErrScriptNotFound    = errors.New("script not found")
)

// ScriptManager manages saving, loading, and deleting payload scripts.
type ScriptManager struct {
	mu         sync.RWMutex
	storageDir string
}

// NewScriptManager initializes the script manager with a target directory.
func NewScriptManager(storageDir string) (*ScriptManager, error) {
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create script storage directory: %w", err)
	}

	sm := &ScriptManager{
		storageDir: storageDir,
	}

	if err := sm.seedDefaultScripts(); err != nil {
		log.Printf("[Scripts] Failed to seed default scripts: %v", err)
	}

	return sm, nil
}

func isScriptFile(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, duckyExt) || strings.HasSuffix(lower, ".js") || strings.HasSuffix(lower, ".txt")
}

func (sm *ScriptManager) loadDescriptionsLocked() map[string]string {
	descriptions := make(map[string]string)
	data, err := os.ReadFile(filepath.Join(sm.storageDir, scriptsMetaFile))
	if err == nil {
		_ = json.Unmarshal(data, &descriptions)
	}
	return descriptions
}

func (sm *ScriptManager) saveDescriptionsLocked(descriptions map[string]string) {
	if data, err := json.MarshalIndent(descriptions, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(sm.storageDir, scriptsMetaFile), data, 0644)
	}
}

func extractDefaultDescription(content, scriptType string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(trimmed), "REM ") {
			if desc := strings.TrimSpace(trimmed[4:]); desc != "" {
				return desc
			}
		}
		if strings.HasPrefix(trimmed, "//") {
			if desc := strings.TrimSpace(strings.TrimPrefix(trimmed, "//")); desc != "" {
				return desc
			}
		}
		break
	}
	return fmt.Sprintf("Saved %s payload", scriptType)
}

// ListScripts returns all saved scripts from disk.
func (sm *ScriptManager) ListScripts() ([]Script, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	entries, err := os.ReadDir(sm.storageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read scripts directory: %w", err)
	}

	descriptions := sm.loadDescriptionsLocked()
	scripts := make([]Script, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !isScriptFile(name) {
			continue
		}

		filePath := filepath.Join(sm.storageDir, name)
		contentBytes, err := os.ReadFile(filePath)
		if err != nil {
			log.Printf("[Scripts] Failed to read script %s: %v", name, err)
			continue
		}

		info, _ := entry.Info()
		modTime := time.Now()
		if info != nil {
			modTime = info.ModTime()
		}

		content := string(contentBytes)
		scriptType := scripting.DetectScriptType("", name)
		desc := descriptions[name]
		if desc == "" {
			desc = extractDefaultDescription(content, scriptType)
		}

		scripts = append(scripts, Script{
			Name:        name,
			Type:        scriptType,
			Content:     content,
			Description: desc,
			UpdatedAt:   modTime,
		})
	}

	return scripts, nil
}

func (sm *ScriptManager) resolveScriptPath(name string) (string, error) {
	if name == "" || name == "." || name == ".." ||
		strings.ContainsAny(name, "/\\\x00") ||
		filepath.Base(name) != name {
		return "", ErrInvalidScriptName
	}

	return filepath.Join(filepath.Clean(sm.storageDir), name), nil
}

func normalizeScriptName(name, scriptType string) string {
	stem := name
	lower := strings.ToLower(name)
	for _, ext := range []string{duckyExt, ".txt", ".js"} {
		if strings.HasSuffix(lower, ext) {
			stem = name[:len(name)-len(ext)]
			break
		}
	}
	if strings.TrimSpace(stem) == "" {
		return ""
	}
	if scripting.DetectScriptType(scriptType, name) == scripting.TypeJavaScript {
		return stem + ".js"
	}
	if strings.HasSuffix(lower, ".txt") {
		return stem + ".txt"
	}
	return stem + duckyExt
}

// SaveScript writes a script to disk and returns its normalized filename.
func (sm *ScriptManager) SaveScript(s Script) (string, error) {
	normalized := normalizeScriptName(s.Name, s.Type)
	filePath, err := sm.resolveScriptPath(normalized)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(s.Content) == "" {
		return "", errors.New("script content cannot be empty")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if err := os.WriteFile(filePath, []byte(s.Content), 0644); err != nil {
		return "", fmt.Errorf("failed to write script file: %w", err)
	}

	if strings.TrimSpace(s.Description) != "" {
		descriptions := sm.loadDescriptionsLocked()
		descriptions[normalized] = strings.TrimSpace(s.Description)
		sm.saveDescriptionsLocked(descriptions)
	}

	return normalized, nil
}

// DeleteScript removes a script from disk.
func (sm *ScriptManager) DeleteScript(name string) error {
	filePath, err := sm.resolveScriptPath(name)
	if err != nil {
		return err
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("%w: '%s'", ErrScriptNotFound, name)
	}

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete script file: %w", err)
	}

	descriptions := sm.loadDescriptionsLocked()
	if _, ok := descriptions[name]; ok {
		delete(descriptions, name)
		sm.saveDescriptionsLocked(descriptions)
	}

	return nil
}

func (sm *ScriptManager) seedDefaultScripts() error {
	entries, err := os.ReadDir(sm.storageDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && isScriptFile(entry.Name()) {
				return nil // Directory already contains scripts
			}
		}
	}

	defaults := []Script{
		{
			Name: "windows_reverse_shell.ducky",
			Type: "duckyscript",
			Content: "REM Windows Powershell Launcher\n" +
				defaultDelay1000 +
				"GUI r\n" +
				"DELAY 500\n" +
				"STRING powershell -NoP -NonI -W Hidden -Exec Bypass -Command \"Write-Host Raspiducky Payload Executed!\"\n" +
				defaultEnter,
		},
		{
			Name: "macos_terminal_opener.ducky",
			Type: "duckyscript",
			Content: "REM macOS Spotlight Launcher\n" +
				defaultDelay1000 +
				"GUI SPACE\n" +
				"DELAY 500\n" +
				"STRING Terminal\n" +
				defaultEnter +
				defaultDelay1000 +
				"STRING echo \"Raspiducky macOS Payload Active!\"\n" +
				defaultEnter,
		},
		{
			Name: "mouse_jiggler.js",
			Type: "javascript",
			Content: "// Raspiducky JS Mouse Jiggler\n" +
				"console.log(\"Starting Mouse Jiggler loop...\");\n" +
				"for (let i = 0; i < 10; i++) {\n" +
				"    mouseMove(10, 0);\n" +
				defaultJSDelay +
				"    mouseMove(0, 10);\n" +
				defaultJSDelay +
				"    mouseMove(-10, 0);\n" +
				defaultJSDelay +
				"    mouseMove(0, -10);\n" +
				defaultJSDelay +
				"}\n" +
				"console.log(\"Jiggler finished!\");\n",
		},
	}

	for _, d := range defaults {
		_ = os.WriteFile(filepath.Join(sm.storageDir, d.Name), []byte(d.Content), 0644)
	}

	return nil
}
