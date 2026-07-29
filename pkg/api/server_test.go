package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestAPIServer(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "raspiducky-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	server, err := NewServer(ServerOptions{
		StorageDir: tempDir,
	})
	if err != nil {
		t.Fatalf("Failed to initialize server: %v", err)
	}

	handler := server.Handler()

	t.Run("GET /api/gadget", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/gadget", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", rec.Code)
		}

		var status GadgetStatus
		if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
			t.Fatalf("Invalid JSON response: %v", err)
		}

		if !status.Deployed {
			t.Errorf("Expected deployed to be true")
		}
	})

	t.Run("POST /api/gadget", func(t *testing.T) {
		newCfg := GadgetConfig{
			Keyboard:     true,
			Mouse:        false,
			Storage:      true,
			Ethernet:     false,
			Serial:       false,
			VendorID:     "0x1d6b",
			ProductID:    "0x0105",
			Manufacturer: "Test Mfg",
			Product:      "Test Prod",
			SerialNumber: "RPD-TEST-001",
		}
		body, _ := json.Marshal(newCfg)
		req := httptest.NewRequest("POST", "/api/gadget", bytes.NewBuffer(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("POST /api/gadget with Ethernet and Serial", func(t *testing.T) {
		newCfg := GadgetConfig{
			Keyboard:       true,
			Mouse:          true,
			Storage:        false,
			Ethernet:       true,
			Serial:         true,
			VendorID:       "0x1d6b",
			ProductID:      "0x0104",
			Manufacturer:   "Test Mfg",
			Product:        "Test Prod",
			SerialNumber:   "RPD-TEST-002",
			KeyboardLayout: "ES",
			StorageSizeMB:  200,
		}
		body, _ := json.Marshal(newCfg)
		req := httptest.NewRequest("POST", "/api/gadget", bytes.NewBuffer(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		var status GadgetStatus
		if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
			t.Fatalf("Invalid JSON response: %v", err)
		}

		if !status.Config.Ethernet {
			t.Error("Expected ethernet to be true in response")
		}
		if !status.Config.Serial {
			t.Error("Expected serial to be true in response")
		}
	})

	t.Run("Config persistence", func(t *testing.T) {
		cfgPath := tempDir + "/gadget_config.json"
		data, err := os.ReadFile(cfgPath)
		if err != nil {
			t.Fatalf("Expected gadget_config.json to exist: %v", err)
		}

		var savedCfg GadgetConfig
		if err := json.Unmarshal(data, &savedCfg); err != nil {
			t.Fatalf("Invalid saved config JSON: %v", err)
		}

		if savedCfg.KeyboardLayout != "ES" {
			t.Errorf("Expected saved keyboardLayout to be ES, got %s", savedCfg.KeyboardLayout)
		}
		if savedCfg.StorageSizeMB != 200 {
			t.Errorf("Expected saved storageSizeMb to be 200, got %d", savedCfg.StorageSizeMB)
		}
		if !savedCfg.Ethernet {
			t.Error("Expected saved ethernet to be true")
		}
		if !savedCfg.Serial {
			t.Error("Expected saved serial to be true")
		}
	})

	t.Run("GET & POST & DELETE /api/scripts", func(t *testing.T) {
		// List
		req := httptest.NewRequest("GET", "/api/scripts", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", rec.Code)
		}

		// Create
		newScript := Script{
			Name:    "test_payload.ducky",
			Type:    "duckyscript",
			Content: "STRING Hello Test\nENTER\n",
		}
		body, _ := json.Marshal(newScript)
		req = httptest.NewRequest("POST", "/api/scripts", bytes.NewBuffer(body))
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Errorf("Expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
		}

		// Delete
		req = httptest.NewRequest("DELETE", "/api/scripts/test_payload.ducky", nil)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Errorf("Expected 204 No Content, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("POST /api/run & POST /api/stop", func(t *testing.T) {
		runReq := RunRequest{
			Script: "DELAY 2000\nSTRING Running test\n",
			Name:   "unit_test_run.ducky",
			Type:   "duckyscript",
		}
		body, _ := json.Marshal(runReq)
		req := httptest.NewRequest("POST", "/api/run", bytes.NewBuffer(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		// Stop
		req = httptest.NewRequest("POST", "/api/stop", nil)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("GET / (Embedded static web files)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected 200 OK for static index.html, got %d", rec.Code)
		}
	})
}
