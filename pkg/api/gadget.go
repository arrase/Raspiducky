package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/arrase/Raspiducky/pkg/gadget"
)

// GadgetManager manages USB gadget configuration and status.
type GadgetManager struct {
	mu            sync.RWMutex
	currentStatus GadgetStatus
	hub           *Hub
	gm            *gadget.GadgetManager
}

// NewGadgetManager initializes a new GadgetManager instance.
func NewGadgetManager(hub *Hub) *GadgetManager {
	manager := &GadgetManager{
		hub: hub,
		gm:  gadget.NewGadgetManager(),
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
				Product:      "Raspiducky Multi-Function HID",
				SerialNumber: "RPD-2026-0001",
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
