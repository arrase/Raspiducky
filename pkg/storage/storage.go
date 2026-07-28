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

type Storage struct {
	baseDir    string
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
		baseDir:    baseDir,
		scriptsDir: scriptsDir,
		configsDir: configsDir,
	}, nil
}

func (s *Storage) SaveScript(name, scriptType, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return errors.New("invalid script name")
	}

	ext := ".js"
	if scriptType == "ducky" || strings.HasSuffix(name, ".txt") {
		ext = ".txt"
	}

	cleanName := strings.TrimSuffix(name, ".js")
	cleanName = strings.TrimSuffix(cleanName, ".txt") + ext

	filePath := filepath.Join(s.scriptsDir, cleanName)
	return os.WriteFile(filePath, []byte(content), 0644)
}

func (s *Storage) LoadScript(name string) (*ScriptItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return nil, errors.New("invalid script name")
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
		content, _ := os.ReadFile(filepath.Join(s.scriptsDir, name))
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

	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return errors.New("invalid script name")
	}

	return os.Remove(filepath.Join(s.scriptsDir, name))
}

func (s *Storage) SaveConfig(name string, configData interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.Contains(name, "..") || strings.Contains(name, "/") {
		return errors.New("invalid profile name")
	}

	data, err := json.MarshalIndent(configData, "", "  ")
	if err != nil {
		return err
	}

	filePath := filepath.Join(s.configsDir, name+".json")
	return os.WriteFile(filePath, data, 0644)
}

func (s *Storage) LoadConfig(name string, target interface{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filePath := filepath.Join(s.configsDir, name+".json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, target)
}
