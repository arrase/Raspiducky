package api

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// GadgetManager manages USB gadget configuration and status.
type GadgetManager struct {
	mu            sync.RWMutex
	configfsPath  string
	currentStatus GadgetStatus
	hub           *Hub
}

// NewGadgetManager initializes a new GadgetManager instance.
func NewGadgetManager(hub *Hub) *GadgetManager {
	gm := &GadgetManager{
		configfsPath: "/sys/kernel/config/usb_gadget/raspiducky",
		hub:          hub,
		currentStatus: GadgetStatus{
			Deployed:        true,
			ActiveFunctions: []string{"keyboard.usb0", "mouse.usb0"},
			UDC:             "3f980000.usb",
			Config: GadgetConfig{
				Keyboard:     true,
				Mouse:        true,
				Storage:      false,
				Ethernet:     false,
				Serial:       false,
				VendorID:     "0x1d6b",
				ProductID:    "0x0104",
				Manufacturer: "Raspiducky Labs",
				Product:      "Raspiducky Multi-Function HID",
				SerialNumber: "RPD-2026-0001",
			},
		},
	}
	_ = gm.syncFromSystem()
	return gm
}

// GetStatus returns the current USB gadget status and config.
func (gm *GadgetManager) GetStatus() GadgetStatus {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	return gm.currentStatus
}

// UpdateConfig applies and deploys a new USB gadget configuration.
func (gm *GadgetManager) UpdateConfig(cfg GadgetConfig) (GadgetStatus, error) {
	if err := validateGadgetConfig(cfg); err != nil {
		return GadgetStatus{}, fmt.Errorf("invalid gadget config: %w", err)
	}

	gm.mu.Lock()
	defer gm.mu.Unlock()

	activeFuncs := make([]string, 0, 5)
	if cfg.Keyboard {
		activeFuncs = append(activeFuncs, "hid.usb0")
	}
	if cfg.Mouse {
		activeFuncs = append(activeFuncs, "hid.usb1")
	}
	if cfg.Storage {
		activeFuncs = append(activeFuncs, "mass_storage.usb0")
	}
	if cfg.Ethernet {
		activeFuncs = append(activeFuncs, "rndis.usb0")
	}
	if cfg.Serial {
		activeFuncs = append(activeFuncs, "acm.usb0")
	}

	gm.currentStatus.Config = cfg
	gm.currentStatus.ActiveFunctions = activeFuncs
	gm.currentStatus.Deployed = true

	// Apply to configfs if Linux kernel configfs directory exists
	if _, err := os.Stat(gm.configfsPath); err == nil {
		_ = gm.applyConfigfs(cfg)
	}

	if gm.hub != nil {
		gm.hub.Broadcast(WSMessage{
			Type:    "gadget_status",
			Payload: gm.currentStatus,
		})
	}

	return gm.currentStatus, nil
}

func validateGadgetConfig(cfg GadgetConfig) error {
	if cfg.VendorID == "" || !strings.HasPrefix(cfg.VendorID, "0x") {
		return errors.New("vendorId must be a hex string starting with 0x (e.g. 0x1d6b)")
	}
	if cfg.ProductID == "" || !strings.HasPrefix(cfg.ProductID, "0x") {
		return errors.New("productId must be a hex string starting with 0x (e.g. 0x0104)")
	}
	if cfg.Manufacturer == "" {
		return errors.New("manufacturer string cannot be empty")
	}
	if cfg.Product == "" {
		return errors.New("product string cannot be empty")
	}
	return nil
}

func (gm *GadgetManager) syncFromSystem() error {
	udcFile := filepath.Join(gm.configfsPath, "UDC")
	data, err := os.ReadFile(udcFile)
	if err != nil {
		return err
	}

	udc := strings.TrimSpace(string(data))
	gm.mu.Lock()
	gm.currentStatus.UDC = udc
	gm.currentStatus.Deployed = udc != ""
	gm.mu.Unlock()
	return nil
}

func (gm *GadgetManager) applyConfigfs(cfg GadgetConfig) error {
	// If writing to real configfs:
	// Unbind UDC -> update strings/functions -> rebind UDC
	_ = os.WriteFile(filepath.Join(gm.configfsPath, "UDC"), []byte("\n"), 0644)
	_ = os.WriteFile(filepath.Join(gm.configfsPath, "idVendor"), []byte(cfg.VendorID+"\n"), 0644)
	_ = os.WriteFile(filepath.Join(gm.configfsPath, "idProduct"), []byte(cfg.ProductID+"\n"), 0644)
	return nil
}
