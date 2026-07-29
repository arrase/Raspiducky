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

	ErrGadgetNotDeployed = errors.New("gadget is not currently deployed")
	ErrUDCNotFound       = errors.New("no UDC driver found in system")
)

// KeyboardReportDesc returns a cloned byte slice of the standard HID keyboard report descriptor.
func KeyboardReportDesc() []byte {
	desc := make([]byte, len(keyboardReportDesc))
	copy(desc, keyboardReportDesc)
	return desc
}

// MouseReportDesc returns a cloned byte slice of the standard HID mouse report descriptor.
func MouseReportDesc() []byte {
	desc := make([]byte, len(mouseReportDesc))
	copy(desc, mouseReportDesc)
	return desc
}

type GadgetManager struct {
	baseDir    string
	gadgetName string
	udcDir     string
	debugFSDir string
	mu         sync.Mutex
	currentCfg Config
	deployed   bool
}

type Option func(*GadgetManager)

func WithBaseDir(dir string) Option {
	return func(gm *GadgetManager) {
		gm.baseDir = dir
	}
}

func WithGadgetName(name string) Option {
	return func(gm *GadgetManager) {
		gm.gadgetName = name
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
		gadgetName: DefaultGadgetName,
		udcDir:     DefaultUDCDir,
		debugFSDir: DefaultDebugFSDir,
	}
	for _, opt := range opts {
		opt(gm)
	}
	return gm
}

func (gm *GadgetManager) GadgetPath() string {
	return filepath.Join(gm.baseDir, gm.gadgetName)
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
		if err := writeFile(filepath.Join(gadgetPath, w.relPath), []byte(w.val)); err != nil {
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
		if err := writeFile(filepath.Join(strDir, w.name), []byte(w.val)); err != nil {
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
	if err := writeFile(filepath.Join(cfgStrDir, "configuration"), []byte("Config 1: Composite")); err != nil {
		return "", err
	}
	if err := writeFile(filepath.Join(cfgDir, "MaxPower"), []byte(DefaultMaxPower)); err != nil {
		return "", err
	}
	if err := writeFile(filepath.Join(cfgDir, "bmAttributes"), []byte(DefaultBmAttributes)); err != nil {
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
		if err := gm.setupKeyboard(gadgetPath, cfgDir); err != nil {
			return err
		}
	}
	if cfg.Mouse {
		if err := gm.setupMouse(gadgetPath, cfgDir); err != nil {
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
		bindErr = writeFile(udcPath, []byte(udcName))
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

	// Always teardown any active gadget before deploying new configuration
	if err := gm.destroyGadgetUnlocked(ctx); err != nil {
		return fmt.Errorf("failed tearing down old gadget: %w", err)
	}

	gadgetPath := gm.GadgetPath()
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

	if err := gm.bindUDC(ctx, gadgetPath); err != nil {
		return err
	}

	gm.currentCfg = cfg
	gm.deployed = true
	return nil
}

func (gm *GadgetManager) setupHID(gadgetPath, cfgDir, name, protocol, reportLen string, reportDesc []byte) error {
	funcDir := filepath.Join(gadgetPath, "functions", name)
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		return fmt.Errorf("failed creating %s function: %w", name, err)
	}
	if err := writeFile(filepath.Join(funcDir, "protocol"), []byte(protocol)); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(funcDir, "subclass"), []byte("1")); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(funcDir, "report_length"), []byte(reportLen)); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(funcDir, "report_desc"), reportDesc); err != nil {
		return err
	}
	return os.Symlink(funcDir, filepath.Join(cfgDir, name))
}

func (gm *GadgetManager) setupKeyboard(gadgetPath, cfgDir string) error {
	return gm.setupHID(gadgetPath, cfgDir, "hid.usb0", "1", "8", KeyboardReportDesc())
}

func (gm *GadgetManager) setupMouse(gadgetPath, cfgDir string) error {
	return gm.setupHID(gadgetPath, cfgDir, "hid.usb1", "2", "6", MouseReportDesc())
}

func (gm *GadgetManager) setupMassStorage(gadgetPath, cfgDir string, ms MassStorageConfig) error {
	funcDir := filepath.Join(gadgetPath, "functions", "mass_storage.usb0")
	lunDir := filepath.Join(funcDir, "lun.0")
	if err := os.MkdirAll(lunDir, 0755); err != nil {
		return fmt.Errorf("failed creating mass_storage.usb0 lun.0: %w", err)
	}
	if err := writeFile(filepath.Join(funcDir, "stall"), []byte("1")); err != nil {
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

	if err := writeFile(filepath.Join(lunDir, "cdrom"), []byte(cdromVal)); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(lunDir, "ro"), []byte(roVal)); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(lunDir, "removable"), []byte("1")); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(lunDir, "nofua"), []byte("0")); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(lunDir, "file"), []byte(ms.BackingFile)); err != nil {
		return err
	}
	return os.Symlink(funcDir, filepath.Join(cfgDir, "mass_storage.usb0"))
}

func (gm *GadgetManager) setupRNDIS(gadgetPath, cfgDir string, rndis EthernetConfig) error {
	funcDir := filepath.Join(gadgetPath, "functions", "rndis.usb0")
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		return fmt.Errorf("failed creating rndis.usb0 function: %w", err)
	}
	if err := writeFile(filepath.Join(funcDir, "host_addr"), []byte(rndis.HostAddr)); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(funcDir, "dev_addr"), []byte(rndis.DevAddr)); err != nil {
		return err
	}

	// Windows OS descriptors
	osDescDir := filepath.Join(gadgetPath, "os_desc")
	if err := os.MkdirAll(osDescDir, 0755); err != nil {
		return fmt.Errorf("failed creating os_desc dir: %w", err)
	}
	if err := writeFile(filepath.Join(osDescDir, "use"), []byte(RndisOSDescUse)); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(osDescDir, "b_vendor_code"), []byte(RndisOSDescVendorCode)); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(osDescDir, "qw_sign"), []byte(RndisOSDescQWSign)); err != nil {
		return err
	}

	ifaceDir := filepath.Join(funcDir, "os_desc", "interface.rndis")
	if err := os.MkdirAll(ifaceDir, 0755); err != nil {
		return fmt.Errorf("failed creating rndis os_desc interface: %w", err)
	}
	if err := writeFile(filepath.Join(ifaceDir, "compatible_id"), []byte(RndisOSDescCompatID)); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(ifaceDir, "sub_compatible_id"), []byte(RndisOSDescSubCompatID)); err != nil {
		return err
	}

	if err := os.Symlink(cfgDir, filepath.Join(osDescDir, "c.1")); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed symlinking config to os_desc: %w", err)
	}

	return os.Symlink(funcDir, filepath.Join(cfgDir, "rndis.usb0"))
}

func (gm *GadgetManager) setupECM(gadgetPath, cfgDir string, ecm EthernetConfig) error {
	funcDir := filepath.Join(gadgetPath, "functions", "ecm.usb0")
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		return fmt.Errorf("failed creating ecm.usb0 function: %w", err)
	}
	if err := writeFile(filepath.Join(funcDir, "host_addr"), []byte(ecm.HostAddr)); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(funcDir, "dev_addr"), []byte(ecm.DevAddr)); err != nil {
		return err
	}
	return os.Symlink(funcDir, filepath.Join(cfgDir, "ecm.usb0"))
}

func (gm *GadgetManager) setupACM(gadgetPath, cfgDir string) error {
	funcDir := filepath.Join(gadgetPath, "functions", "acm.usb0")
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		return fmt.Errorf("failed creating acm.usb0 function: %w", err)
	}
	return os.Symlink(funcDir, filepath.Join(cfgDir, "acm.usb0"))
}

func (gm *GadgetManager) SetMassStorageFile(ctx context.Context, backingFile string, cdrom bool) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	if !gm.deployed {
		return ErrGadgetNotDeployed
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	lunDir := filepath.Join(gm.GadgetPath(), "functions", "mass_storage.usb0", "lun.0")
	if _, err := os.Stat(lunDir); os.IsNotExist(err) {
		return errors.New("mass storage function is not configured")
	}

	cdromVal := "0"
	roVal := "0"
	if cdrom {
		cdromVal = "1"
		roVal = "1"
	}
	if err := writeFile(filepath.Join(lunDir, "cdrom"), []byte(cdromVal)); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(lunDir, "ro"), []byte(roVal)); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(lunDir, "file"), []byte(backingFile)); err != nil {
		return err
	}

	gm.currentCfg.MassStorage.BackingFile = backingFile
	gm.currentCfg.MassStorage.CDROM = cdrom
	return nil
}

func (gm *GadgetManager) DestroyGadget(ctx context.Context) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	return gm.destroyGadgetUnlocked(ctx)
}

func (gm *GadgetManager) unbindUDC() error {
	udcPath := filepath.Join(gm.GadgetPath(), "UDC")
	data, err := os.ReadFile(udcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(data) > 1 {
		if err := writeFile(udcPath, []byte("\n")); err != nil {
			return err
		}
		// Allow kernel time to fully release the USB device controller
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

func cleanupDir(dir string) error {
	var errs []error
	if entries, err := os.ReadDir(dir); err == nil {
		for _, entry := range entries {
			p := filepath.Join(dir, entry.Name())
			if err := os.RemoveAll(p); err != nil && !os.IsNotExist(err) {
				errs = append(errs, err)
			}
		}
		if err := os.Remove(dir); err != nil && !os.IsNotExist(err) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (gm *GadgetManager) cleanupConfigsDir() error {
	var errs []error
	cfgDir := filepath.Join(gm.GadgetPath(), "configs", "c.1")

	// Clean up config strings directory first if it exists
	if err := cleanupDir(filepath.Join(cfgDir, "strings")); err != nil {
		errs = append(errs, err)
	}

	// Remove all remaining entries (symlinks, attribute files) inside cfgDir
	if err := cleanupDir(cfgDir); err != nil {
		errs = append(errs, err)
	}

	if err := os.Remove(filepath.Join(gm.GadgetPath(), "configs")); err != nil && !os.IsNotExist(err) {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func (gm *GadgetManager) cleanupOSDescDir() error {
	return cleanupDir(filepath.Join(gm.GadgetPath(), "os_desc"))
}

func (gm *GadgetManager) cleanupFunctionsDir() error {
	return cleanupDir(filepath.Join(gm.GadgetPath(), "functions"))
}

func (gm *GadgetManager) cleanupStringsDir() error {
	return cleanupDir(filepath.Join(gm.GadgetPath(), "strings"))
}

func (gm *GadgetManager) destroyGadgetUnlocked(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	gadgetPath := gm.GadgetPath()
	if _, err := os.Stat(gadgetPath); os.IsNotExist(err) {
		gm.deployed = false
		return nil
	}

	var errs []error

	// Step 1: Unbind UDC
	if err := gm.unbindUDC(); err != nil {
		errs = append(errs, err)
	}

	// Step 2: Clear configuration symlinks & subdirs
	if err := gm.cleanupConfigsDir(); err != nil {
		errs = append(errs, err)
	}

	// Step 3: Clear OS descriptors symlinks & files
	if err := gm.cleanupOSDescDir(); err != nil {
		errs = append(errs, err)
	}

	// Step 4: Remove functions
	if err := gm.cleanupFunctionsDir(); err != nil {
		errs = append(errs, err)
	}

	// Step 5: Remove strings
	if err := gm.cleanupStringsDir(); err != nil {
		errs = append(errs, err)
	}

	// Step 6: Remove gadget root folder
	if err := os.RemoveAll(gadgetPath); err != nil && !os.IsNotExist(err) {
		errs = append(errs, fmt.Errorf("failed removing gadget directory %s: %w", gadgetPath, err))
	}

	gm.deployed = false
	gm.currentCfg = Config{}
	return errors.Join(errs...)
}

func (gm *GadgetManager) State() (Config, bool) {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	return gm.currentCfg, gm.deployed
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}
