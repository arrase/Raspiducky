package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type sampleConfig struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Count   int    `json:"count"`
}

func TestStorage_NewStorage(t *testing.T) {
	_, err := NewStorage("")
	if !errors.Is(err, ErrEmptyBaseDir) {
		t.Fatalf("expected ErrEmptyBaseDir, got %v", err)
	}

	tmpDir := t.TempDir()
	s, err := NewStorage(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error creating storage: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil Storage instance")
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "scripts")); err != nil {
		t.Errorf("scripts directory was not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "configs")); err != nil {
		t.Errorf("configs directory was not created: %v", err)
	}
}

func TestStorage_ScriptsLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := NewStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Validation checks
	if err := s.SaveScript("../bad.js", "js", "content"); !errors.Is(err, ErrInvalidName) {
		t.Errorf("expected ErrInvalidName for path traversal, got %v", err)
	}
	if err := s.SaveScript("invalid_type.py", "python", "content"); !errors.Is(err, ErrInvalidType) {
		t.Errorf("expected ErrInvalidType, got %v", err)
	}
	if _, err := s.LoadScript("sub/dir.js"); !errors.Is(err, ErrInvalidName) {
		t.Errorf("expected ErrInvalidName on LoadScript with slash, got %v", err)
	}
	if err := s.DeleteScript(""); !errors.Is(err, ErrInvalidName) {
		t.Errorf("expected ErrInvalidName on DeleteScript with empty name, got %v", err)
	}

	// Save JS script
	jsCode := "console.log('hello');"
	if err := s.SaveScript("test1.js", "js", jsCode); err != nil {
		t.Fatalf("failed to save js script: %v", err)
	}

	// Save Ducky script
	duckyCode := "STRING Hello World\nENTER"
	if err := s.SaveScript("test2", "ducky", duckyCode); err != nil {
		t.Fatalf("failed to save ducky script: %v", err)
	}

	// Save script with multiple extensions stripping
	if err := s.SaveScript("test3.txt.ducky.js", "js", "var x = 1;"); err != nil {
		t.Fatalf("failed to save test3: %v", err)
	}

	// Load scripts
	item1, err := s.LoadScript("test1.js")
	if err != nil {
		t.Fatalf("failed to load test1.js: %v", err)
	}
	if item1.Type != "js" || item1.Content != jsCode {
		t.Errorf("unexpected content for test1: %+v", item1)
	}

	item2, err := s.LoadScript("test2.txt")
	if err != nil {
		t.Fatalf("failed to load test2.txt: %v", err)
	}
	if item2.Type != "ducky" || item2.Content != duckyCode {
		t.Errorf("unexpected content for test2: %+v", item2)
	}

	// Non-existent script
	if _, err := s.LoadScript("notfound.js"); err == nil {
		t.Error("expected error loading non-existent script")
	}

	// List scripts
	list, err := s.ListScripts()
	if err != nil {
		t.Fatalf("failed to list scripts: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("expected 3 scripts, got %d", len(list))
	}

	// Delete scripts
	if err := s.DeleteScript("test1.js"); err != nil {
		t.Fatalf("failed to delete test1.js: %v", err)
	}
	if err := s.DeleteScript("test1.js"); err == nil {
		t.Error("expected error deleting already deleted script")
	}

	remaining, err := s.ListScripts()
	if err != nil {
		t.Fatalf("failed to list scripts after deletion: %v", err)
	}
	if len(remaining) != 2 {
		t.Errorf("expected 2 scripts remaining, got %d", len(remaining))
	}
}

func TestStorage_ConfigsLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := NewStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	cfg := sampleConfig{
		Name:    "Raspiducky",
		Enabled: true,
		Count:   42,
	}

	// Validation
	if err := s.SaveConfig("../invalid.json", cfg); !errors.Is(err, ErrInvalidName) {
		t.Errorf("expected ErrInvalidName, got %v", err)
	}
	var target sampleConfig
	if err := s.LoadConfig("../invalid.json", &target); !errors.Is(err, ErrInvalidName) {
		t.Errorf("expected ErrInvalidName, got %v", err)
	}

	// Save config
	if err := s.SaveConfig("settings", cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Load config
	var loaded sampleConfig
	if err := s.LoadConfig("settings.json", &loaded); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if loaded != cfg {
		t.Errorf("loaded config %+v does not match saved %+v", loaded, cfg)
	}

	// Load non-existent
	var missing sampleConfig
	if err := s.LoadConfig("unknown", &missing); err == nil {
		t.Error("expected error loading unknown config")
	}
}
