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
	duckyExt        = ".ducky"
	scriptsMetaFile = "scripts_meta.json"
)

// Script validation and lookup errors.
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

func parseCommentLine(trimmed string) (string, bool) {
	if len(trimmed) >= 4 && strings.EqualFold(trimmed[:4], "REM ") {
		return strings.TrimSpace(trimmed[4:]), true
	}
	for _, prefix := range []string{"//", "#"} {
		if after, ok := strings.CutPrefix(trimmed, prefix); ok {
			return strings.TrimSpace(after), true
		}
	}
	return "", false
}

func extractDefaultDescription(content, scriptType string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		comment, isComment := parseCommentLine(trimmed)
		if !isComment {
			break
		}
		if comment != "" {
			return comment
		}
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
		if entry.IsDir() || !isScriptFile(entry.Name()) {
			continue
		}

		name := entry.Name()
		contentBytes, err := os.ReadFile(filepath.Join(sm.storageDir, name))
		if err != nil {
			log.Printf("[Scripts] Failed to read script %s: %v", name, err)
			continue
		}

		modTime := time.Now()
		if info, infoErr := entry.Info(); infoErr == nil && info != nil {
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

	if _, err := os.Stat(filePath); errors.Is(err, os.ErrNotExist) {
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
	if entries, err := os.ReadDir(sm.storageDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && isScriptFile(entry.Name()) {
				return nil
			}
		}
	}

	defaults := []Script{
		{
			Name: "windows_reverse_shell.ducky",
			Type: scripting.TypeDuckyScript,
			Content: `REM Windows Powershell Launcher
DELAY 1000
GUI r
DELAY 500
STRING powershell -NoP -NonI -W Hidden -Exec Bypass -Command "Write-Host Raspiducky Payload Executed!"
ENTER
`,
		},
		{
			Name: "macos_terminal_opener.ducky",
			Type: scripting.TypeDuckyScript,
			Content: `REM macOS Spotlight Launcher
DELAY 1000
GUI SPACE
DELAY 500
STRING Terminal
ENTER
DELAY 1000
STRING echo "Raspiducky macOS Payload Active!"
ENTER
`,
		},
		{
			Name: "mouse_jiggler.js",
			Type: scripting.TypeJavaScript,
			Content: `// Raspiducky JS Mouse Jiggler
console.log("Starting Mouse Jiggler loop...");
for (let i = 0; i < 10; i++) {
    mouseMove(10, 0);
    delay(200);
    mouseMove(0, 10);
    delay(200);
    mouseMove(-10, 0);
    delay(200);
    mouseMove(0, -10);
    delay(200);
}
console.log("Jiggler finished!");
`,
		},
	}

	for _, d := range defaults {
		if err := os.WriteFile(filepath.Join(sm.storageDir, d.Name), []byte(d.Content), 0644); err != nil {
			return err
		}
	}
	return nil
}
