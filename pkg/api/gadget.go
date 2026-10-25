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
	*gadget.GadgetManager
	mu            sync.RWMutex
	currentStatus GadgetStatus
	hub           *Hub
	runner        *RunnerEngine
	keyboard      *hid.Keyboard
	mouse         *hid.Mouse
	ledWatcher    *hid.LEDWatcher
	devDir        string
	storageDir    string
}

// NewGadgetManager initializes a new GadgetManager instance.
func NewGadgetManager(hub *Hub, runner *RunnerEngine, opts ServerOptions) *GadgetManager {
	baseGM := opts.GadgetManager
	if baseGM == nil {
		baseGM = gadget.NewGadgetManager()
	}

	devDir := "/dev"
	if opts.Keyboard.DevicePath() != "" {
		devDir = filepath.Dir(opts.Keyboard.DevicePath())
	}

	initialCfg := GadgetConfig{
		Keyboard:       true,
		Mouse:          true,
		Storage:        false,
		RNDIS:          false,
		ECM:            false,
		Serial:         false,
		VendorID:       "0x1d6b",
		ProductID:      "0x0104",
		Manufacturer:   "Raspiducky Labs",
		Product:        "Raspiducky Multi-Function HID",
		SerialNumber:   "RPD-2026-0001",
		StorageSizeMB:  100,
		KeyboardLayout: "US",
		TypingDelayMs:  10,
	}

	if opts.StorageDir != "" {
		_ = os.MkdirAll(opts.StorageDir, 0755)
		cfgPath := filepath.Join(opts.StorageDir, "gadget_config.json")
		if data, err := os.ReadFile(cfgPath); err == nil {
			savedCfg := initialCfg
			if err := json.Unmarshal(data, &savedCfg); err == nil {
				if savedCfg.Ethernet && !savedCfg.RNDIS && !savedCfg.ECM {
					savedCfg.RNDIS = true
					savedCfg.ECM = true
				}
				initialCfg = savedCfg
			}
		}
	}

	if opts.KeyboardLayout != "" {
		initialCfg.KeyboardLayout = opts.KeyboardLayout
	}
	if opts.TypingDelayMs > 0 {
		initialCfg.TypingDelayMs = opts.TypingDelayMs
	}

	manager := &GadgetManager{
		GadgetManager: baseGM,
		hub:           hub,
		runner:        runner,
		keyboard:      opts.Keyboard,
		mouse:         opts.Mouse,
		ledWatcher:    opts.LEDWatcher,
		devDir:        devDir,
		storageDir:    opts.StorageDir,
		currentStatus: GadgetStatus{
			Deployed:        false,
			ActiveFunctions: []string{},
			UDC:             "",
			EndpointCosts:   gadget.EndpointCosts(),
			Config:          initialCfg,
		},
	}

	if _, err := manager.UpdateConfig(manager.currentStatus.Config); err != nil {
		log.Printf("[Gadget] Could not deploy initial gadget config: %v", err)
	}

	return manager
}

func (gm *GadgetManager) refreshHardwareStatus(status *GadgetStatus) {
	if udc, err := gm.GetUDCName(); err == nil {
		status.UDC = udc
	} else {
		status.UDC = ""
	}
	if maxEp, err := gm.GetMaxEndpoints(); err == nil {
		status.MaxEndpoints = maxEp
	} else {
		status.MaxEndpoints = 0
	}
	status.EndpointCosts = gadget.EndpointCosts()
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
	if cfg.Ethernet && !cfg.RNDIS && !cfg.ECM {
		cfg.RNDIS = true
		cfg.ECM = true
	}
	cfg.Ethernet = cfg.RNDIS || cfg.ECM
	if cfg.KeyboardLayout == "" {
		cfg.KeyboardLayout = "US"
	}
	if cfg.StorageSizeMB <= 0 {
		cfg.StorageSizeMB = 100
	}

	if _, err := hid.GetLayout(cfg.KeyboardLayout); err != nil {
		return GadgetStatus{}, fmt.Errorf("invalid keyboard layout: %w", err)
	}

	gadgetCfg := buildGadgetConfig(cfg, gm.storageDir)
	if err := gm.ValidateHardware(gadgetCfg); err != nil {
		return GadgetStatus{}, fmt.Errorf("invalid gadget config: %w", err)
	}

	_ = gm.runner.Stop()

	gm.mu.Lock()
	defer gm.mu.Unlock()

	if cfg.Storage {
		if err := ensureBackingFile(gadgetCfg.MassStorage.BackingFile, cfg.StorageSizeMB); err != nil {
			return GadgetStatus{}, fmt.Errorf("failed to ensure mass storage backing file: %w", err)
		}
	}

	_ = gm.keyboard.Close()
	_ = gm.mouse.Close()
	if gm.ledWatcher != nil {
		gm.ledWatcher.SetDevicePath("")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := gm.DeployValidated(ctx, gadgetCfg); err != nil {
		return GadgetStatus{}, fmt.Errorf("failed to deploy configfs: %w", err)
	}

	_ = gm.keyboard.SetLayout(cfg.KeyboardLayout)
	gm.keyboard.SetTypingSpeed(cfg.TypingDelayMs, 0)
	if cfg.Keyboard {
		hidg0 := filepath.Join(gm.devDir, "hidg0")
		gm.keyboard.SetDevicePath(hidg0)
		if gm.ledWatcher != nil {
			gm.ledWatcher.SetDevicePath(hidg0)
		}
	} else {
		gm.keyboard.SetDevicePath("")
	}

	if !cfg.Mouse {
		gm.mouse.SetDevicePath("")
	} else if !cfg.Keyboard {
		gm.mouse.SetDevicePath(filepath.Join(gm.devDir, "hidg0"))
	} else {
		gm.mouse.SetDevicePath(filepath.Join(gm.devDir, "hidg1"))
	}

	activeFuncs := gadgetCfg.ActiveFunctions()
	gm.currentStatus.Config = cfg
	gm.currentStatus.ActiveFunctions = activeFuncs
	gm.currentStatus.Deployed = len(activeFuncs) > 0
	gm.currentStatus.UsedEndpoints = gadgetCfg.CountEndpoints()

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

func buildGadgetConfig(cfg GadgetConfig, storageDir string) gadget.Config {
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
			CDROM:       cfg.StorageCDROM,
			ReadOnly:    cfg.StorageReadOnly,
		}
	}
	if cfg.RNDIS {
		hostAddr := cfg.RNDISHostAddr
		if hostAddr == "" {
			hostAddr = "02:00:00:00:00:01"
		}
		devAddr := cfg.RNDISDevAddr
		if devAddr == "" {
			devAddr = "02:00:00:00:00:02"
		}
		gadgetCfg.RNDIS = gadget.EthernetConfig{
			Enabled:  true,
			HostAddr: hostAddr,
			DevAddr:  devAddr,
		}
	}
	if cfg.ECM {
		hostAddr := cfg.ECMHostAddr
		if hostAddr == "" {
			hostAddr = "02:00:00:00:00:03"
		}
		devAddr := cfg.ECMDevAddr
		if devAddr == "" {
			devAddr = "02:00:00:00:00:04"
		}
		gadgetCfg.ECM = gadget.EthernetConfig{
			Enabled:  true,
			HostAddr: hostAddr,
			DevAddr:  devAddr,
		}
	}
	if cfg.Serial {
		gadgetCfg.ACM = gadget.SerialConfig{
			Enabled: true,
		}
	}

	return gadgetCfg
}

func ensureBackingFile(path string, sizeMB int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	if info, err := os.Stat(path); err == nil {
		if info.Size() > 0 {
			return nil
		}
		_ = os.Remove(path)
	}

	targetSize := int64(sizeMB) * 1024 * 1024
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
