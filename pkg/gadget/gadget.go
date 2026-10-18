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
	baseDir    string
	udcDir     string
	debugFSDir string
	mu         sync.Mutex
}

type Option func(*GadgetManager)

func WithBaseDir(dir string) Option {
	return func(gm *GadgetManager) {
		gm.baseDir = dir
	}
}

func WithUDCDir(dir string) Option {
	return func(gm *GadgetManager) {
		gm.udcDir = dir
	}
}

func WithDebugFSDir(dir string) Option {
	return func(gm *GadgetManager) {
		gm.debugFSDir = dir
	}
}

func NewGadgetManager(opts ...Option) *GadgetManager {
	gm := &GadgetManager{
		baseDir:    DefaultBaseDir,
		udcDir:     DefaultUDCDir,
		debugFSDir: DefaultDebugFSDir,
	}
	for _, opt := range opts {
		opt(gm)
	}
	return gm
}

func (gm *GadgetManager) GadgetPath() string {
	return filepath.Join(gm.baseDir, DefaultGadgetName)
}

func (gm *GadgetManager) GetUDCName() (string, error) {
	entries, err := os.ReadDir(gm.udcDir)
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

	hwParamsPath := filepath.Join(gm.debugFSDir, udcName, "hw_params")
	data, err := os.ReadFile(hwParamsPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read hw_params (is debugfs mounted at /sys/kernel/debug?): %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
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

func (gm *GadgetManager) validateAndCheckHardware(cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid gadget config: %w", err)
	}

	maxEp, err := gm.GetMaxEndpoints()
	if err != nil {
		return fmt.Errorf("critical: cannot verify hardware endpoint limits: %w", err)
	}

	if epCount := cfg.CountINEndpoints(); epCount > maxEp {
		return fmt.Errorf("configuration requires %d IN endpoints, but the hardware only supports max %d", epCount, maxEp)
	}
	return nil
}

func (gm *GadgetManager) writeBaseAttributes(gadgetPath string, cfg Config) error {
	writes := []struct {
		relPath string
		val     string
	}{
		{"idVendor", cfg.VID},
		{"idProduct", cfg.PID},
		{"bcdUSB", DefaultBcdUSB},
		{"bcdDevice", DefaultBcdDevice},
		{"bDeviceClass", DefaultBDeviceClass},
		{"bDeviceSubClass", DefaultBDeviceSubClass},
		{"bDeviceProtocol", DefaultBDeviceProtocol},
	}

	for _, w := range writes {
		if err := os.WriteFile(filepath.Join(gadgetPath, w.relPath), []byte(w.val), 0644); err != nil {
			return err
		}
	}
	return nil
}

func (gm *GadgetManager) writeStringDescriptors(gadgetPath string, cfg Config) error {
	strDir := filepath.Join(gadgetPath, "strings", "0x409")
	if err := os.MkdirAll(strDir, 0755); err != nil {
		return fmt.Errorf("failed creating strings dir: %w", err)
	}
	strWrites := []struct {
		name string
		val  string
	}{
		{"serialnumber", cfg.Serial},
		{"manufacturer", cfg.Manufacturer},
		{"product", cfg.Product},
	}
	for _, w := range strWrites {
		if err := os.WriteFile(filepath.Join(strDir, w.name), []byte(w.val), 0644); err != nil {
			return err
		}
	}
	return nil
}

func (gm *GadgetManager) setupConfigNode(gadgetPath string) (string, error) {
	cfgDir := filepath.Join(gadgetPath, "configs", "c.1")
	cfgStrDir := filepath.Join(cfgDir, "strings", "0x409")
	if err := os.MkdirAll(cfgStrDir, 0755); err != nil {
		return "", fmt.Errorf("failed creating config string dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(cfgStrDir, "configuration"), []byte("Config 1: Composite"), 0644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "MaxPower"), []byte(DefaultMaxPower), 0644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "bmAttributes"), []byte(DefaultBmAttributes), 0644); err != nil {
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

	if err := gm.validateAndCheckHardware(cfg); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	gadgetPath := gm.GadgetPath()
	// Always teardown any active gadget before deploying new configuration
	if err := gm.destroyGadgetUnlocked(gadgetPath); err != nil {
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
	if err := os.WriteFile(filepath.Join(funcDir, "protocol"), []byte(protocol), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(funcDir, "subclass"), []byte("1"), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(funcDir, "report_length"), []byte(reportLen), 0644); err != nil {
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
	if err := os.WriteFile(filepath.Join(funcDir, "stall"), []byte("1"), 0644); err != nil {
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

	if err := os.WriteFile(filepath.Join(lunDir, "cdrom"), []byte(cdromVal), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(lunDir, "ro"), []byte(roVal), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(lunDir, "removable"), []byte("1"), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(lunDir, "nofua"), []byte("0"), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(lunDir, "file"), []byte(ms.BackingFile), 0644); err != nil {
		return err
	}
	return os.Symlink(funcDir, filepath.Join(cfgDir, FuncMassStorage))
}

func (gm *GadgetManager) setupRNDIS(gadgetPath, cfgDir string, rndis EthernetConfig) error {
	funcDir := filepath.Join(gadgetPath, "functions", FuncRNDIS)
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		return fmt.Errorf("failed creating %s function: %w", FuncRNDIS, err)
	}
	if err := os.WriteFile(filepath.Join(funcDir, "host_addr"), []byte(rndis.HostAddr), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(funcDir, "dev_addr"), []byte(rndis.DevAddr), 0644); err != nil {
		return err
	}

	// Windows OS descriptors
	osDescDir := filepath.Join(gadgetPath, "os_desc")
	if err := os.MkdirAll(osDescDir, 0755); err != nil {
		return fmt.Errorf("failed creating os_desc dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(osDescDir, "use"), []byte(RndisOSDescUse), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(osDescDir, "b_vendor_code"), []byte(RndisOSDescVendorCode), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(osDescDir, "qw_sign"), []byte(RndisOSDescQWSign), 0644); err != nil {
		return err
	}

	ifaceDir := filepath.Join(funcDir, "os_desc", "interface.rndis")
	if err := os.MkdirAll(ifaceDir, 0755); err != nil {
		return fmt.Errorf("failed creating rndis os_desc interface: %w", err)
	}
	if err := os.WriteFile(filepath.Join(ifaceDir, "compatible_id"), []byte(RndisOSDescCompatID), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(ifaceDir, "sub_compatible_id"), []byte(RndisOSDescSubCompatID), 0644); err != nil {
		return err
	}

	if err := os.Symlink(cfgDir, filepath.Join(osDescDir, "c.1")); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed symlinking config to os_desc: %w", err)
	}

	return os.Symlink(funcDir, filepath.Join(cfgDir, FuncRNDIS))
}

func (gm *GadgetManager) setupECM(gadgetPath, cfgDir string, ecm EthernetConfig) error {
	funcDir := filepath.Join(gadgetPath, "functions", FuncECM)
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		return fmt.Errorf("failed creating %s function: %w", FuncECM, err)
	}
	if err := os.WriteFile(filepath.Join(funcDir, "host_addr"), []byte(ecm.HostAddr), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(funcDir, "dev_addr"), []byte(ecm.DevAddr), 0644); err != nil {
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

func removeConfigfsDir(dir string) error {
	if err := os.Remove(dir); err == nil || os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(dir)
}

func (gm *GadgetManager) destroyGadgetUnlocked(gadgetPath string) error {
	if _, err := os.Stat(gadgetPath); os.IsNotExist(err) {
		return nil
	}

	// 1. Unbind UDC
	udcPath := filepath.Join(gadgetPath, "UDC")
	if _, err := os.Stat(udcPath); err == nil {
		if err := os.WriteFile(udcPath, []byte(""), 0644); err != nil {
			return err
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
	if err := removeConfigfsDir(filepath.Join(cfgDir, "strings", "0x409")); err != nil {
		return err
	}
	if err := removeConfigfsDir(cfgDir); err != nil {
		return err
	}

	// 5. Remove each function directory inside functions/
	functionsDir := filepath.Join(gadgetPath, "functions")
	if entries, err := os.ReadDir(functionsDir); err == nil {
		for _, entry := range entries {
			if err := removeConfigfsDir(filepath.Join(functionsDir, entry.Name())); err != nil {
				return err
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	// 6. Remove strings/0x409 and gadgetPath
	if err := removeConfigfsDir(filepath.Join(gadgetPath, "strings", "0x409")); err != nil {
		return err
	}
	return removeConfigfsDir(gadgetPath)
}
