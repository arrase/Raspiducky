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

var ErrInvalidName = errors.New("invalid file name")

func isValidFilename(name string) bool {
	return name != "" && filepath.Base(name) == name && !strings.Contains(name, "..")
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
	if baseDir == "" {
		baseDir = "/var/lib/raspiducky"
	}

	scriptsDir := filepath.Join(baseDir, "scripts")
	configsDir := filepath.Join(baseDir, "configs")

	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create scripts directory: %w", err)
	}
	if err := os.MkdirAll(configsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create configs directory: %w", err)
	}

	return &Storage{
		scriptsDir: scriptsDir,
		configsDir: configsDir,
	}, nil
}

func (s *Storage) SaveScript(name, scriptType, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !isValidFilename(name) {
		return ErrInvalidName
	}

	ext := ".js"
	if scriptType == "ducky" || strings.HasSuffix(name, ".txt") {
		ext = ".txt"
	}

	cleanName := strings.TrimSuffix(name, ".js")
	cleanName = strings.TrimSuffix(cleanName, ".txt") + ext

	filePath := filepath.Join(s.scriptsDir, cleanName)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing script %q: %w", cleanName, err)
	}
	return nil
}

func (s *Storage) LoadScript(name string) (*ScriptItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !isValidFilename(name) {
		return nil, ErrInvalidName
	}

	filePath := filepath.Join(s.scriptsDir, name)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	scriptType := "js"
	if strings.HasSuffix(name, ".txt") {
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
		return nil, err
	}

	var items []ScriptItem
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		scriptType := "js"
		if strings.HasSuffix(name, ".txt") {
			scriptType = "ducky"
		}
		content, err := os.ReadFile(filepath.Join(s.scriptsDir, name))
		if err != nil {
			return nil, fmt.Errorf("reading script %q: %w", name, err)
		}
		items = append(items, ScriptItem{
			Name:    name,
			Type:    scriptType,
			Content: string(content),
		})
	}
	return items, nil
}

func (s *Storage) DeleteScript(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !isValidFilename(name) {
		return ErrInvalidName
	}

	if err := os.Remove(filepath.Join(s.scriptsDir, name)); err != nil {
		return fmt.Errorf("deleting script %q: %w", name, err)
	}
	return nil
}

func (s *Storage) SaveConfig(name string, configData interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !isValidFilename(name) {
		return ErrInvalidName
	}

	data, err := json.MarshalIndent(configData, "", "  ")
	if err != nil {
		return err
	}

	filePath := filepath.Join(s.configsDir, name+".json")
	return os.WriteFile(filePath, data, 0644)
}

func (s *Storage) LoadConfig(name string, target interface{}) error {
	if !isValidFilename(name) {
		return ErrInvalidName
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	filePath := filepath.Join(s.configsDir, name+".json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, target)
}
