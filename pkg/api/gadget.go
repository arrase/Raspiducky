package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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
	mouse         *hid.Mouse
	storageDir    string
}

// NewGadgetManager initializes a new GadgetManager instance.
func NewGadgetManager(hub *Hub, keyboard *hid.Keyboard, mouse *hid.Mouse, storageDir string, gadgetOpts ...gadget.Option) *GadgetManager {
	initialCfg := GadgetConfig{
		Keyboard:       true,
		Mouse:          true,
		Storage:        false,
		Ethernet:       false,
		Serial:         false,
		VendorID:       "0x1d6b",
		ProductID:      "0x0104",
		Manufacturer:   "Raspiducky Labs",
		Product:        "Raspiducky Multi-Function HID",
		SerialNumber:   "RPD-2026-0001",
		StorageSizeMB:  100,
		KeyboardLayout: "US",
	}

	if storageDir != "" {
		_ = os.MkdirAll(storageDir, 0755)
		cfgPath := filepath.Join(storageDir, "gadget_config.json")
		if data, err := os.ReadFile(cfgPath); err == nil {
			var savedCfg GadgetConfig
			if err := json.Unmarshal(data, &savedCfg); err == nil {
				initialCfg = savedCfg
			}
		}
	}

	manager := &GadgetManager{
		hub:        hub,
		gm:         gadget.NewGadgetManager(gadgetOpts...),
		keyboard:   keyboard,
		mouse:      mouse,
		storageDir: storageDir,
		currentStatus: GadgetStatus{
			Deployed:        false,
			ActiveFunctions: []string{},
			UDC:             "",
			Config:          initialCfg,
		},
	}

	// Deploy config at startup
	if _, err := manager.UpdateConfig(manager.currentStatus.Config); err != nil {
		log.Printf("[Gadget] Could not deploy initial gadget config: %v", err)
	}

	return manager
}

func (gm *GadgetManager) refreshHardwareStatus(status *GadgetStatus) {
	if udc, err := gm.gm.GetUDCName(); err == nil {
		status.UDC = udc
	} else {
		status.UDC = ""
	}
	if maxEp, err := gm.gm.GetMaxEndpoints(); err == nil {
		status.MaxEndpoints = maxEp
	} else {
		status.MaxEndpoints = 0
	}
}

// GetStatus returns the current USB gadget status and config.
func (gm *GadgetManager) GetStatus() GadgetStatus {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	status := gm.currentStatus
	status.ActiveFunctions = make([]string, len(gm.currentStatus.ActiveFunctions))
	copy(status.ActiveFunctions, gm.currentStatus.ActiveFunctions)

	gm.refreshHardwareStatus(&status)

	return status
}

// UpdateConfig applies and deploys a new USB gadget configuration.
func (gm *GadgetManager) UpdateConfig(cfg GadgetConfig) (GadgetStatus, error) {
	gadgetCfg, activeFuncs := buildGadgetConfig(cfg, gm.storageDir)
	if err := gadgetCfg.Validate(); err != nil {
		return GadgetStatus{}, fmt.Errorf("invalid gadget config: %w", err)
	}
	if cfg.KeyboardLayout != "" {
		if _, err := hid.GetLayout(cfg.KeyboardLayout); err != nil {
			return GadgetStatus{}, fmt.Errorf("invalid keyboard layout: %w", err)
		}
	}

	gm.mu.Lock()
	defer gm.mu.Unlock()

	if cfg.Storage {
		size := cfg.StorageSizeMB
		if size <= 0 {
			size = 100
		}
		if err := ensureBackingFile(gadgetCfg.MassStorage.BackingFile, size); err != nil {
			return GadgetStatus{}, fmt.Errorf("failed to ensure mass storage backing file: %w", err)
		}
	}

	if cfg.KeyboardLayout != "" && gm.keyboard != nil {
		if err := gm.keyboard.SetLayout(cfg.KeyboardLayout); err != nil {
			return GadgetStatus{}, fmt.Errorf("failed to set keyboard layout: %w", err)
		}
	}

	if gm.keyboard != nil {
		_ = gm.keyboard.Close()
	}
	if gm.mouse != nil {
		_ = gm.mouse.Close()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := gm.gm.Deploy(ctx, gadgetCfg); err != nil {
		return GadgetStatus{}, fmt.Errorf("failed to deploy configfs: %w", err)
	}

	if gm.mouse != nil {
		if !cfg.Keyboard && cfg.Mouse {
			gm.mouse.SetDevicePath("/dev/hidg0")
		} else {
			gm.mouse.SetDevicePath("/dev/hidg1")
		}
	}

	gm.currentStatus.Config = cfg
	gm.currentStatus.ActiveFunctions = activeFuncs
	gm.currentStatus.Deployed = len(activeFuncs) > 0

	if gm.storageDir != "" {
		if data, err := json.MarshalIndent(cfg, "", "  "); err == nil {
			_ = os.WriteFile(filepath.Join(gm.storageDir, "gadget_config.json"), data, 0644)
		}
	}

	gm.refreshHardwareStatus(&gm.currentStatus)

	gm.hub.Broadcast(WSMessage{
		Type:    "gadget_status",
		Payload: gm.currentStatus,
	})

	return gm.currentStatus, nil
}

func buildGadgetConfig(cfg GadgetConfig, storageDir string) (gadget.Config, []string) {
	activeFuncs := make([]string, 0, 7)
	if cfg.Keyboard {
		activeFuncs = append(activeFuncs, gadget.FuncKeyboard)
	}
	if cfg.Mouse {
		activeFuncs = append(activeFuncs, gadget.FuncMouse)
	}
	if cfg.Storage {
		activeFuncs = append(activeFuncs, gadget.FuncMassStorage)
	}
	if cfg.Ethernet {
		activeFuncs = append(activeFuncs, gadget.FuncRNDIS, gadget.FuncECM)
	}
	if cfg.Serial {
		activeFuncs = append(activeFuncs, gadget.FuncACM)
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
		diskPath := "/var/lib/raspiducky/disk.img"
		if storageDir != "" {
			diskPath = filepath.Join(storageDir, "disk.img")
		}
		gadgetCfg.MassStorage = gadget.MassStorageConfig{
			Enabled:     true,
			BackingFile: diskPath,
		}
	}
	if cfg.Ethernet {
		gadgetCfg.RNDIS = gadget.EthernetConfig{
			Enabled:  true,
			HostAddr: "02:00:00:00:00:01",
			DevAddr:  "02:00:00:00:00:02",
		}
		gadgetCfg.ECM = gadget.EthernetConfig{
			Enabled:  true,
			HostAddr: "02:00:00:00:00:03",
			DevAddr:  "02:00:00:00:00:04",
		}
	}
	if cfg.Serial {
		gadgetCfg.ACM = gadget.SerialConfig{
			Enabled: true,
		}
	}

	return gadgetCfg, activeFuncs
}

func ensureBackingFile(path string, sizeMB int) error {
	targetSize := int64(sizeMB) * 1024 * 1024
	if info, err := os.Stat(path); err == nil {
		if info.Size() == targetSize && targetSize > 0 {
			return nil
		}
		_ = os.Remove(path)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	if err := f.Truncate(targetSize); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return err
	}
	_ = f.Close()

	mkfsPath, err := exec.LookPath("mkfs.vfat")
	if err != nil {
		if _, statErr := os.Stat("/usr/sbin/mkfs.vfat"); statErr == nil {
			mkfsPath = "/usr/sbin/mkfs.vfat"
		} else {
			_ = os.Remove(path)
			return fmt.Errorf("mkfs.vfat executable not found: %w", err)
		}
	}
	cmd := exec.Command(mkfsPath, path)
	if err := cmd.Run(); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("formatting disk image with %s: %w", mkfsPath, err)
	}
	return nil
}
