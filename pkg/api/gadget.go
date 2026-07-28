package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/arrase/Raspiducky/pkg/gadget"
	"github.com/arrase/Raspiducky/pkg/hid"
)

// GadgetManager manages USB gadget configuration and status.
type GadgetManager struct {
	mu            sync.RWMutex
	currentStatus GadgetStatus
	hub           *Hub
	gm            *gadget.GadgetManager
	keyboard      *hid.Keyboard
}

// NewGadgetManager initializes a new GadgetManager instance.
func NewGadgetManager(hub *Hub, keyboard *hid.Keyboard) *GadgetManager {
	manager := &GadgetManager{
		hub:      hub,
		gm:       gadget.NewGadgetManager(),
		keyboard: keyboard,
		currentStatus: GadgetStatus{
			Deployed:        false,
			ActiveFunctions: []string{},
			UDC:             "",
			Config: GadgetConfig{
				Keyboard:     true,
				Mouse:        true,
				Storage:      false,
				Ethernet:     false,
				Serial:       false,
				VendorID:     "0x1d6b",
				ProductID:    "0x0104",
				Manufacturer: "Raspiducky Labs",
				Product:        "Raspiducky Multi-Function HID",
				SerialNumber:   "RPD-2026-0001",
				StorageSizeMB:  100,
				KeyboardLayout: "US",
			},
		},
	}
	
	// Deploy default config at startup
	if _, err := manager.UpdateConfig(manager.currentStatus.Config); err != nil {
		log.Printf("[Gadget] Could not deploy default gadget config: %v", err)
	}
	
	return manager
}

// GetStatus returns the current USB gadget status and config.
func (gm *GadgetManager) GetStatus() GadgetStatus {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	
	if udc, err := gm.gm.GetUDCName(); err == nil {
		gm.currentStatus.UDC = udc
	}
	
	return gm.currentStatus
}

// UpdateConfig applies and deploys a new USB gadget configuration.
func (gm *GadgetManager) UpdateConfig(cfg GadgetConfig) (GadgetStatus, error) {
	if err := validateGadgetConfig(cfg); err != nil {
		return GadgetStatus{}, fmt.Errorf("invalid gadget config: %w", err)
	}

	gm.mu.Lock()
	defer gm.mu.Unlock()

	if cfg.KeyboardLayout != "" && gm.keyboard != nil {
		if err := gm.keyboard.SetLayout(cfg.KeyboardLayout); err != nil {
			return GadgetStatus{}, fmt.Errorf("failed to set keyboard layout: %w", err)
		}
	}

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

	gadgetCfg := gadget.Config{
		VID:          cfg.VendorID,
		PID:          cfg.ProductID,
		Manufacturer: cfg.Manufacturer,
		Product:      cfg.Product,
		Serial:       cfg.SerialNumber,
		Keyboard:     cfg.Keyboard,
		Mouse:        cfg.Mouse,
	}
	
	if cfg.Storage {
		size := cfg.StorageSizeMB
		if size <= 0 {
			size = 100 // Default to 100MB if invalid or 0
		}
		if err := ensureBackingFile("/var/lib/raspiducky/disk.img", size); err != nil {
			return GadgetStatus{}, fmt.Errorf("failed to ensure mass storage backing file: %w", err)
		}
		gadgetCfg.MassStorage = gadget.MassStorageConfig{
			Enabled: true,
			BackingFile: "/var/lib/raspiducky/disk.img",
		}
	}
	if cfg.Ethernet {
		gadgetCfg.RNDIS = gadget.EthernetConfig{
			Enabled: true,
			HostAddr: "02:00:00:00:00:01",
			DevAddr: "02:00:00:00:00:02",
		}
	}
	if cfg.Serial {
		gadgetCfg.ACM = gadget.SerialConfig{
			Enabled: true,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := gm.gm.Deploy(ctx, gadgetCfg); err != nil {
		return GadgetStatus{}, fmt.Errorf("failed to deploy configfs: %w", err)
	}

	gm.currentStatus.Config = cfg
	gm.currentStatus.ActiveFunctions = activeFuncs
	gm.currentStatus.Deployed = true
	
	if udc, err := gm.gm.GetUDCName(); err == nil {
		gm.currentStatus.UDC = udc
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

func ensureBackingFile(path string, sizeMB int) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		// Create file of sizeMB
		if err := f.Truncate(int64(sizeMB) * 1024 * 1024); err != nil {
			f.Close()
			return err
		}
		f.Close()

		cmd := exec.Command("/usr/sbin/mkfs.vfat", path)
		if err := cmd.Run(); err != nil {
			// fallback if mkfs.vfat is in PATH instead
			cmd2 := exec.Command("mkfs.vfat", path)
			if err2 := cmd2.Run(); err2 != nil {
				return fmt.Errorf("formatting disk image: %v, %v", err, err2)
			}
		}
	}
	return nil
}
