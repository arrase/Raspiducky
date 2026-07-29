package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	ErrInvalidName  = errors.New("invalid file name")
	ErrInvalidType  = errors.New("invalid script type: must be 'js' or 'ducky'")
	ErrEmptyBaseDir = errors.New("base directory cannot be empty")
)

func isValidFilename(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	if strings.ContainsAny(name, "/\\\x00") || strings.Contains(name, "..") {
		return false
	}
	return filepath.Base(name) == name
}

type Storage struct {
	scriptsDir string
	configsDir string
	mu         sync.RWMutex
}

type ScriptItem struct {
	Name    string `json:"name"`
	Type    string `json:"type"` // "js" or "ducky"
	Content string `json:"content"`
}

func NewStorage(baseDir string) (*Storage, error) {
	if strings.TrimSpace(baseDir) == "" {
		return nil, ErrEmptyBaseDir
	}

	scriptsDir := filepath.Join(baseDir, "scripts")
	configsDir := filepath.Join(baseDir, "configs")

	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return nil, fmt.Errorf("creating scripts directory: %w", err)
	}
	if err := os.MkdirAll(configsDir, 0755); err != nil {
		return nil, fmt.Errorf("creating configs directory: %w", err)
	}

	return &Storage{
		scriptsDir: scriptsDir,
		configsDir: configsDir,
	}, nil
}

func writeAtomic(dir, filename string, data []byte, perm os.FileMode) error {
	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmpFile.Chmod(perm); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("setting temp file permissions: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("syncing temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	targetPath := filepath.Join(dir, filename)
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}

func (s *Storage) SaveScript(name, scriptType, content string) error {
	if !isValidFilename(name) {
		return ErrInvalidName
	}
	if scriptType != "js" && scriptType != "ducky" {
		return ErrInvalidType
	}

	ext := ".js"
	if scriptType == "ducky" {
		ext = ".txt"
	}

	base := name
	for {
		if strings.HasSuffix(base, ".js") {
			base = strings.TrimSuffix(base, ".js")
		} else if strings.HasSuffix(base, ".txt") {
			base = strings.TrimSuffix(base, ".txt")
		} else if strings.HasSuffix(base, ".ducky") {
			base = strings.TrimSuffix(base, ".ducky")
		} else {
			break
		}
	}
	cleanName := base + ext

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := writeAtomic(s.scriptsDir, cleanName, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing script %q: %w", cleanName, err)
	}
	return nil
}

func (s *Storage) LoadScript(name string) (*ScriptItem, error) {
	if !isValidFilename(name) {
		return nil, ErrInvalidName
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	filePath := filepath.Join(s.scriptsDir, name)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading script %q: %w", name, err)
	}

	scriptType := "js"
	if strings.HasSuffix(name, ".txt") || strings.HasSuffix(name, ".ducky") {
		scriptType = "ducky"
	}

	return &ScriptItem{
		Name:    name,
		Type:    scriptType,
		Content: string(content),
	}, nil
}

func (s *Storage) ListScripts() ([]ScriptItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.scriptsDir)
	if err != nil {
		return nil, fmt.Errorf("listing scripts directory: %w", err)
	}

	var items []ScriptItem
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".js") && !strings.HasSuffix(name, ".txt") && !strings.HasSuffix(name, ".ducky") {
			continue
		}
		scriptType := "js"
		if strings.HasSuffix(name, ".txt") || strings.HasSuffix(name, ".ducky") {
			scriptType = "ducky"
		}

		filePath := filepath.Join(s.scriptsDir, name)
		content, err := os.ReadFile(filePath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("reading script %q: %w", name, err)
		}

		items = append(items, ScriptItem{
			Name:    name,
			Type:    scriptType,
			Content: string(content),
		})
	}
	if items == nil {
		items = []ScriptItem{}
	}
	return items, nil
}

func (s *Storage) DeleteScript(name string) error {
	if !isValidFilename(name) {
		return ErrInvalidName
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := filepath.Join(s.scriptsDir, name)
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("deleting script %q: %w", name, err)
	}
	return nil
}

func (s *Storage) SaveConfig(name string, configData interface{}) error {
	if !isValidFilename(name) {
		return ErrInvalidName
	}

	cleanName := strings.TrimSuffix(name, ".json") + ".json"

	data, err := json.MarshalIndent(configData, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config %q: %w", cleanName, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := writeAtomic(s.configsDir, cleanName, data, 0644); err != nil {
		return fmt.Errorf("writing config %q: %w", cleanName, err)
	}
	return nil
}

func (s *Storage) LoadConfig(name string, target interface{}) error {
	if !isValidFilename(name) {
		return ErrInvalidName
	}

	cleanName := strings.TrimSuffix(name, ".json") + ".json"

	s.mu.RLock()
	defer s.mu.RUnlock()

	filePath := filepath.Join(s.configsDir, cleanName)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading config %q: %w", name, err)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("unmarshaling config %q: %w", name, err)
	}
	return nil
}
