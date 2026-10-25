package gadget

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	DefaultBaseDir    = "/sys/kernel/config/usb_gadget"
	DefaultGadgetName = "raspiducky"
	DefaultUDCDir     = "/sys/class/udc"
	DefaultDebugFSDir = "/sys/kernel/debug/usb"

	DefaultBcdDevice = "0x0100"
	DefaultBcdUSB    = "0x0200"

	DefaultBDeviceClass    = "0xEF"
	DefaultBDeviceSubClass = "0x02"
	DefaultBDeviceProtocol = "0x01"

	DefaultMaxPower     = "250"
	DefaultBmAttributes = "0x80"

	RndisOSDescUse         = "1"
	RndisOSDescVendorCode  = "0xbc"
	RndisOSDescQWSign      = "MSFT100"
	RndisOSDescCompatID    = "RNDIS"
	RndisOSDescSubCompatID = "5162001"

	FuncKeyboard    = "hid.usb0"
	FuncMouse       = "hid.usb1"
	FuncMassStorage = "mass_storage.usb0"
	FuncRNDIS       = "rndis.usb0"
	FuncECM         = "ecm.usb0"
	FuncACM         = "acm.usb0"
)

var (
	// keyboardReportDesc standard HID 8-byte keyboard report descriptor
	keyboardReportDesc = []byte{
		0x05, 0x01, 0x09, 0x06, 0xa1, 0x01, 0x05, 0x07, 0x19, 0xe0, 0x29, 0xe7, 0x15, 0x00, 0x25, 0x01,
		0x75, 0x01, 0x95, 0x08, 0x81, 0x02, 0x95, 0x01, 0x75, 0x08, 0x81, 0x03, 0x95, 0x05, 0x75, 0x01,
		0x05, 0x08, 0x19, 0x01, 0x29, 0x05, 0x91, 0x02, 0x95, 0x01, 0x75, 0x03, 0x91, 0x03, 0x95, 0x06,
		0x75, 0x08, 0x15, 0x00, 0x25, 0x65, 0x05, 0x07, 0x19, 0x00, 0x29, 0x65, 0x81, 0x00, 0xc0,
	}

	// mouseReportDesc standard HID 6-byte mouse report descriptor
	mouseReportDesc = []byte{
		0x05, 0x01, 0x09, 0x02, 0xa1, 0x01, 0x09, 0x01, 0xa1, 0x00, 0x85, 0x01, 0x05, 0x09, 0x19, 0x01,
		0x29, 0x03, 0x15, 0x00, 0x25, 0x01, 0x95, 0x03, 0x75, 0x01, 0x81, 0x02, 0x95, 0x01, 0x75, 0x05,
		0x81, 0x03, 0x05, 0x01, 0x09, 0x30, 0x09, 0x31, 0x15, 0x81, 0x25, 0x7f, 0x75, 0x08, 0x95, 0x02,
		0x81, 0x06, 0x95, 0x02, 0x75, 0x08, 0x81, 0x01, 0xc0, 0xc0, 0x05, 0x01, 0x09, 0x02, 0xa1, 0x01,
		0x09, 0x01, 0xa1, 0x00, 0x85, 0x02, 0x05, 0x09, 0x19, 0x01, 0x29, 0x03, 0x15, 0x00, 0x25, 0x01,
		0x95, 0x03, 0x75, 0x01, 0x81, 0x02, 0x95, 0x01, 0x75, 0x05, 0x81, 0x01, 0x05, 0x01, 0x09, 0x30,
		0x09, 0x31, 0x15, 0x00, 0x26, 0xff, 0x7f, 0x95, 0x02, 0x75, 0x10, 0x81, 0x02, 0xc0, 0xc0,
	}

	ErrUDCNotFound = errors.New("no UDC driver found in system")
)

type GadgetManager struct {
	BaseDir    string
	UDCDir     string
	DebugFSDir string
	mu         sync.Mutex
}

func NewGadgetManager() *GadgetManager {
	return &GadgetManager{
		BaseDir:    DefaultBaseDir,
		UDCDir:     DefaultUDCDir,
		DebugFSDir: DefaultDebugFSDir,
	}
}

// NewMockGadgetManager creates a mock UDC, debugfs, and usb_gadget hierarchy under rootDir for testing.
func NewMockGadgetManager(rootDir, maxEndpoints string) (*GadgetManager, error) {
	baseDir := filepath.Join(rootDir, "usb_gadget")
	udcDir := filepath.Join(rootDir, "udc")
	debugFSDir := filepath.Join(rootDir, "debugfs")

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(udcDir, 0755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(udcDir, "dummy.udc"), []byte(""), 0644); err != nil {
		return nil, err
	}
	if maxEndpoints != "" {
		udcDebugDir := filepath.Join(debugFSDir, "dummy.udc")
		if err := os.MkdirAll(udcDebugDir, 0755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(udcDebugDir, "hw_params"), []byte("num_dev_ep: "+maxEndpoints+"\n"), 0644); err != nil {
			return nil, err
		}
	}

	return &GadgetManager{
		BaseDir:    baseDir,
		UDCDir:     udcDir,
		DebugFSDir: debugFSDir,
	}, nil
}

func (gm *GadgetManager) GadgetPath() string {
	return filepath.Join(gm.BaseDir, DefaultGadgetName)
}

func (gm *GadgetManager) GetUDCName() (string, error) {
	entries, err := os.ReadDir(gm.UDCDir)
	if err != nil || len(entries) == 0 {
		return "", ErrUDCNotFound
	}
	return entries[0].Name(), nil
}

func (gm *GadgetManager) GetMaxEndpoints() (int, error) {
	udcName, err := gm.GetUDCName()
	if err != nil {
		return 0, fmt.Errorf("failed to get UDC name: %w", err)
	}

	hwParamsPath := filepath.Join(gm.DebugFSDir, udcName, "hw_params")
	data, err := os.ReadFile(hwParamsPath)
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "num_dev_ep") {
				parts := strings.Split(line, ":")
				if len(parts) == 2 {
					val, err := strconv.Atoi(strings.TrimSpace(parts[1]))
					if err != nil {
						return 0, fmt.Errorf("invalid num_dev_ep value format: %w", err)
					}
					return val, nil
				}
			}
		}
		return 0, errors.New("num_dev_ep not found in hw_params")
	}

	if !os.IsNotExist(err) {
		return 0, fmt.Errorf("failed to read hw_params: %w", err)
	}

	// Non-dwc2 controllers (dwc3, sunxi-musb, dummy_hcd) do not expose dwc2 hw_params.
	// Check sysfs /sys/class/udc/<udc>/ep* if directory entries exist; otherwise return 0 (no fixed dwc2 limit).
	udcSysPath := filepath.Join(gm.UDCDir, udcName)
	if entries, readErr := os.ReadDir(udcSysPath); readErr == nil {
		epCount := 0
		hasEP0 := false
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "ep") {
				epCount++
				if e.Name() == "ep0" {
					hasEP0 = true
				}
			}
		}
		if epCount > 0 {
			if !hasEP0 {
				epCount++
			}
			return epCount, nil
		}
	}

	return 0, nil
}

// ValidateHardware validates the configuration and verifies UDC availability and hardware endpoint limits.
func (gm *GadgetManager) ValidateHardware(cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid gadget config: %w", err)
	}

	if _, err := gm.GetUDCName(); err != nil {
		return fmt.Errorf("critical: %w", err)
	}

	maxEp, err := gm.GetMaxEndpoints()
	if err != nil {
		return fmt.Errorf("critical: cannot verify hardware endpoint limits: %w", err)
	}

	if maxEp > 0 {
		if epCount := cfg.CountEndpoints(); epCount > maxEp {
			return fmt.Errorf("configuration requires %d endpoints, but the hardware only supports max %d", epCount, maxEp)
		}
	}
	return nil
}

type attr struct {
	name string
	val  string
}

func writeAttrs(dir string, attrs ...attr) error {
	for _, a := range attrs {
		if err := os.WriteFile(filepath.Join(dir, a.name), []byte(a.val), 0644); err != nil {
			return err
		}
	}
	return nil
}

func (gm *GadgetManager) writeBaseAttributes(gadgetPath string, cfg Config) error {
	return writeAttrs(gadgetPath,
		attr{"idVendor", cfg.VID},
		attr{"idProduct", cfg.PID},
		attr{"bcdUSB", DefaultBcdUSB},
		attr{"bcdDevice", DefaultBcdDevice},
		attr{"bDeviceClass", DefaultBDeviceClass},
		attr{"bDeviceSubClass", DefaultBDeviceSubClass},
		attr{"bDeviceProtocol", DefaultBDeviceProtocol},
	)
}

func (gm *GadgetManager) writeStringDescriptors(gadgetPath string, cfg Config) error {
	strDir := filepath.Join(gadgetPath, "strings", "0x409")
	if err := os.MkdirAll(strDir, 0755); err != nil {
		return fmt.Errorf("failed creating strings dir: %w", err)
	}
	return writeAttrs(strDir,
		attr{"serialnumber", cfg.Serial},
		attr{"manufacturer", cfg.Manufacturer},
		attr{"product", cfg.Product},
	)
}

func (gm *GadgetManager) setupConfigNode(gadgetPath string) (string, error) {
	cfgDir := filepath.Join(gadgetPath, "configs", "c.1")
	cfgStrDir := filepath.Join(cfgDir, "strings", "0x409")
	if err := os.MkdirAll(cfgStrDir, 0755); err != nil {
		return "", fmt.Errorf("failed creating config string dir: %w", err)
	}
	if err := writeAttrs(cfgStrDir, attr{"configuration", "Config 1: Composite"}); err != nil {
		return "", err
	}
	if err := writeAttrs(cfgDir,
		attr{"MaxPower", DefaultMaxPower},
		attr{"bmAttributes", DefaultBmAttributes},
	); err != nil {
		return "", err
	}
	return cfgDir, nil
}

func (gm *GadgetManager) setupGadgetFunctions(gadgetPath string, cfgDir string, cfg Config) error {
	// RNDIS function must be linked first if enabled for Windows compatibility
	if cfg.RNDIS.Enabled {
		if err := gm.setupRNDIS(gadgetPath, cfgDir, cfg.RNDIS); err != nil {
			return err
		}
	}
	if cfg.ECM.Enabled {
		if err := gm.setupECM(gadgetPath, cfgDir, cfg.ECM); err != nil {
			return err
		}
	}
	if cfg.Keyboard {
		if err := gm.setupHID(gadgetPath, cfgDir, FuncKeyboard, "1", "8", keyboardReportDesc); err != nil {
			return err
		}
	}
	if cfg.Mouse {
		if err := gm.setupHID(gadgetPath, cfgDir, FuncMouse, "2", "6", mouseReportDesc); err != nil {
			return err
		}
	}
	if cfg.MassStorage.Enabled {
		if err := gm.setupMassStorage(gadgetPath, cfgDir, cfg.MassStorage); err != nil {
			return err
		}
	}
	if cfg.ACM.Enabled {
		if err := gm.setupACM(gadgetPath, cfgDir); err != nil {
			return err
		}
	}
	return nil
}

func (gm *GadgetManager) bindUDC(ctx context.Context, gadgetPath string) error {
	udcName, err := gm.GetUDCName()
	if err != nil {
		return fmt.Errorf("failed finding UDC controller: %w", err)
	}
	udcPath := filepath.Join(gadgetPath, "UDC")

	var bindErr error
	for i := 0; i < 10; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		bindErr = os.WriteFile(udcPath, []byte(udcName), 0644)
		if bindErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return fmt.Errorf("failed binding to UDC %s: %w", udcName, bindErr)
}

func (gm *GadgetManager) Deploy(ctx context.Context, cfg Config) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	if err := gm.ValidateHardware(cfg); err != nil {
		return err
	}

	return gm.deployValidatedUnlocked(ctx, cfg)
}

// DeployValidated deploys a configuration that has already been validated by ValidateHardware.
func (gm *GadgetManager) DeployValidated(ctx context.Context, cfg Config) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	return gm.deployValidatedUnlocked(ctx, cfg)
}

func (gm *GadgetManager) deployValidatedUnlocked(ctx context.Context, cfg Config) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	gadgetPath := gm.GadgetPath()
	// Always teardown any active gadget before deploying new configuration
	if err := gm.destroyGadgetUnlocked(ctx, gadgetPath); err != nil {
		return fmt.Errorf("failed tearing down old gadget: %w", err)
	}

	if err := os.MkdirAll(gadgetPath, 0755); err != nil {
		return fmt.Errorf("failed creating gadget directory %s: %w", gadgetPath, err)
	}

	if err := gm.writeBaseAttributes(gadgetPath, cfg); err != nil {
		return err
	}

	if err := gm.writeStringDescriptors(gadgetPath, cfg); err != nil {
		return err
	}

	cfgDir, err := gm.setupConfigNode(gadgetPath)
	if err != nil {
		return err
	}

	if err := gm.setupGadgetFunctions(gadgetPath, cfgDir, cfg); err != nil {
		return err
	}

	return gm.bindUDC(ctx, gadgetPath)
}

func (gm *GadgetManager) setupHID(gadgetPath, cfgDir, name, protocol, reportLen string, reportDesc []byte) error {
	funcDir := filepath.Join(gadgetPath, "functions", name)
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		return fmt.Errorf("failed creating %s function: %w", name, err)
	}
	if err := writeAttrs(funcDir,
		attr{"protocol", protocol},
		attr{"subclass", "1"},
		attr{"report_length", reportLen},
	); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(funcDir, "report_desc"), reportDesc, 0644); err != nil {
		return err
	}
	return os.Symlink(funcDir, filepath.Join(cfgDir, name))
}

func (gm *GadgetManager) setupMassStorage(gadgetPath, cfgDir string, ms MassStorageConfig) error {
	funcDir := filepath.Join(gadgetPath, "functions", FuncMassStorage)
	lunDir := filepath.Join(funcDir, "lun.0")
	if err := os.MkdirAll(lunDir, 0755); err != nil {
		return fmt.Errorf("failed creating %s lun.0: %w", FuncMassStorage, err)
	}
	if err := writeAttrs(funcDir, attr{"stall", "1"}); err != nil {
		return err
	}
	cdromVal := "0"
	roVal := "0"
	if ms.CDROM {
		cdromVal = "1"
		roVal = "1"
	} else if ms.ReadOnly {
		roVal = "1"
	}

	if err := writeAttrs(lunDir,
		attr{"cdrom", cdromVal},
		attr{"ro", roVal},
		attr{"removable", "1"},
		attr{"nofua", "0"},
		attr{"file", ms.BackingFile},
	); err != nil {
		return err
	}
	return os.Symlink(funcDir, filepath.Join(cfgDir, FuncMassStorage))
}

func (gm *GadgetManager) setupEthernetFunc(gadgetPath, funcName string, eth EthernetConfig) (string, error) {
	funcDir := filepath.Join(gadgetPath, "functions", funcName)
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		return "", fmt.Errorf("failed creating %s function: %w", funcName, err)
	}
	if err := writeAttrs(funcDir,
		attr{"host_addr", eth.HostAddr},
		attr{"dev_addr", eth.DevAddr},
	); err != nil {
		return "", err
	}
	return funcDir, nil
}

func (gm *GadgetManager) setupRNDIS(gadgetPath, cfgDir string, rndis EthernetConfig) error {
	funcDir, err := gm.setupEthernetFunc(gadgetPath, FuncRNDIS, rndis)
	if err != nil {
		return err
	}

	// Windows OS descriptors
	osDescDir := filepath.Join(gadgetPath, "os_desc")
	if err := os.MkdirAll(osDescDir, 0755); err != nil {
		return fmt.Errorf("failed creating os_desc dir: %w", err)
	}
	if err := writeAttrs(osDescDir,
		attr{"use", RndisOSDescUse},
		attr{"b_vendor_code", RndisOSDescVendorCode},
		attr{"qw_sign", RndisOSDescQWSign},
	); err != nil {
		return err
	}

	ifaceDir := filepath.Join(funcDir, "os_desc", "interface.rndis")
	if err := os.MkdirAll(ifaceDir, 0755); err != nil {
		return fmt.Errorf("failed creating rndis os_desc interface: %w", err)
	}
	if err := writeAttrs(ifaceDir,
		attr{"compatible_id", RndisOSDescCompatID},
		attr{"sub_compatible_id", RndisOSDescSubCompatID},
	); err != nil {
		return err
	}

	if err := os.Symlink(cfgDir, filepath.Join(osDescDir, "c.1")); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed symlinking config to os_desc: %w", err)
	}

	return os.Symlink(funcDir, filepath.Join(cfgDir, FuncRNDIS))
}

func (gm *GadgetManager) setupECM(gadgetPath, cfgDir string, ecm EthernetConfig) error {
	funcDir, err := gm.setupEthernetFunc(gadgetPath, FuncECM, ecm)
	if err != nil {
		return err
	}
	return os.Symlink(funcDir, filepath.Join(cfgDir, FuncECM))
}

func (gm *GadgetManager) setupACM(gadgetPath, cfgDir string) error {
	funcDir := filepath.Join(gadgetPath, "functions", FuncACM)
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		return fmt.Errorf("failed creating %s function: %w", FuncACM, err)
	}
	return os.Symlink(funcDir, filepath.Join(cfgDir, FuncACM))
}

func removeSymlinks(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

func (gm *GadgetManager) removeConfigfsDir(dir string) error {
	err := os.Remove(dir)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	if gm.BaseDir != DefaultBaseDir && errors.Is(err, syscall.ENOTEMPTY) {
		entries, readErr := os.ReadDir(dir)
		if readErr == nil {
			for _, e := range entries {
				p := filepath.Join(dir, e.Name())
				if e.IsDir() {
					_ = gm.removeConfigfsDir(p)
				} else {
					_ = os.Remove(p)
				}
			}
		}
		return os.Remove(dir)
	}
	return err
}

func (gm *GadgetManager) destroyGadgetUnlocked(ctx context.Context, gadgetPath string) error {
	if _, err := os.Stat(gadgetPath); os.IsNotExist(err) {
		return nil
	}

	// 1. Unbind UDC
	udcPath := filepath.Join(gadgetPath, "UDC")
	if data, err := os.ReadFile(udcPath); err == nil && strings.TrimSpace(string(data)) != "" {
		if err := os.WriteFile(udcPath, []byte("\n"), 0644); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	// 2. Remove symlinks in os_desc (os_desc/c.1 must be unlinked before configs/c.1 is removed)
	if err := removeSymlinks(filepath.Join(gadgetPath, "os_desc")); err != nil {
		return err
	}

	// 3. Remove function symlinks in configs/c.1
	cfgDir := filepath.Join(gadgetPath, "configs", "c.1")
	if err := removeSymlinks(cfgDir); err != nil {
		return err
	}

	// 4. Remove configs/c.1/strings/0x409 and configs/c.1
	if err := gm.removeConfigfsDir(filepath.Join(cfgDir, "strings", "0x409")); err != nil {
		return err
	}
	if err := gm.removeConfigfsDir(cfgDir); err != nil {
		return err
	}

	// 5. Remove each function directory inside functions/
	functionsDir := filepath.Join(gadgetPath, "functions")
	if entries, err := os.ReadDir(functionsDir); err == nil {
		for _, entry := range entries {
			if err := gm.removeConfigfsDir(filepath.Join(functionsDir, entry.Name())); err != nil {
				return err
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	// 6. Remove strings/0x409 and gadgetPath
	if err := gm.removeConfigfsDir(filepath.Join(gadgetPath, "strings", "0x409")); err != nil {
		return err
	}
	return gm.removeConfigfsDir(gadgetPath)
}
