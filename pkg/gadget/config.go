package gadget

import (
	"errors"
	"fmt"
	"net"
	"regexp"
)

var (
	ErrNoFunctionsEnabled = errors.New("at least one USB function must be enabled")
	hexIDRegex            = regexp.MustCompile(`^0x[0-9a-fA-F]{4}$`)
)

// EthernetConfig holds MAC address configuration for network interfaces (RNDIS/ECM).
type EthernetConfig struct {
	Enabled  bool   `json:"enabled"`
	HostAddr string `json:"host_addr"`
	DevAddr  string `json:"dev_addr"`
}

// Validate checks if the ethernet configuration parameters are valid MAC addresses.
func (c EthernetConfig) Validate(name string) error {
	if !c.Enabled {
		return nil
	}
	if _, err := net.ParseMAC(c.HostAddr); err != nil {
		return fmt.Errorf("%s invalid HostAddr %q: %w", name, c.HostAddr, err)
	}
	if _, err := net.ParseMAC(c.DevAddr); err != nil {
		return fmt.Errorf("%s invalid DevAddr %q: %w", name, c.DevAddr, err)
	}
	return nil
}

// MassStorageConfig holds configuration for USB Mass Storage function.
type MassStorageConfig struct {
	Enabled     bool   `json:"enabled"`
	BackingFile string `json:"backing_file"`
	CDROM       bool   `json:"cdrom"`
	ReadOnly    bool   `json:"read_only"`
}

// Validate checks mass storage configuration parameters.
func (c MassStorageConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.BackingFile == "" {
		return errors.New("mass storage backing file cannot be empty when enabled")
	}
	return nil
}

// SerialConfig holds toggle options for ACM serial function.
type SerialConfig struct {
	Enabled bool `json:"enabled"`
}

// Config represents complete USB gadget profile settings.
type Config struct {
	VID          string            `json:"vid"`
	PID          string            `json:"pid"`
	Manufacturer string            `json:"manufacturer"`
	Product      string            `json:"product"`
	Serial       string            `json:"serial"`
	Keyboard     bool              `json:"keyboard"`
	Mouse        bool              `json:"mouse"`
	MassStorage  MassStorageConfig `json:"mass_storage"`
	RNDIS        EthernetConfig    `json:"rndis"`
	ECM          EthernetConfig    `json:"ecm"`
	ACM          SerialConfig      `json:"acm"`
}

// Validate validates the entire gadget configuration profile.
func (c Config) Validate() error {
	if c.VID == "" || c.PID == "" {
		return errors.New("VID and PID cannot be empty")
	}
	if !hexIDRegex.MatchString(c.VID) {
		return fmt.Errorf("invalid VID format %q, expected format 0x1d6b", c.VID)
	}
	if !hexIDRegex.MatchString(c.PID) {
		return fmt.Errorf("invalid PID format %q, expected format 0x0104", c.PID)
	}
	if c.Manufacturer == "" || c.Product == "" || c.Serial == "" {
		return errors.New("manufacturer, product, and serial string descriptors cannot be empty")
	}

	if !c.Keyboard && !c.Mouse && !c.MassStorage.Enabled && !c.RNDIS.Enabled && !c.ECM.Enabled && !c.ACM.Enabled {
		return ErrNoFunctionsEnabled
	}

	if err := c.RNDIS.Validate("RNDIS"); err != nil {
		return err
	}
	if err := c.ECM.Validate("ECM"); err != nil {
		return err
	}
	if err := c.MassStorage.Validate(); err != nil {
		return err
	}

	return nil
}

const (
	EndpointsEP0         = 1
	EndpointsKeyboard    = 1 // Interrupt IN
	EndpointsMouse       = 1 // Interrupt IN
	EndpointsMassStorage = 2 // Bulk IN + Bulk OUT
	EndpointsRNDIS       = 3 // Interrupt IN + Bulk IN + Bulk OUT
	EndpointsECM         = 3 // Interrupt IN + Bulk IN + Bulk OUT
	EndpointsACM         = 3 // Interrupt IN + Bulk IN + Bulk OUT
)

// EndpointCosts returns the hardware endpoint cost map for USB functions.
func EndpointCosts() map[string]int {
	return map[string]int{
		"ep0":      EndpointsEP0,
		"keyboard": EndpointsKeyboard,
		"mouse":    EndpointsMouse,
		"storage":  EndpointsMassStorage,
		"rndis":    EndpointsRNDIS,
		"ecm":      EndpointsECM,
		"serial":   EndpointsACM,
		"ethernet": EndpointsRNDIS + EndpointsECM,
	}
}

// ActiveFunctions returns the list of enabled configfs function names.
func (c Config) ActiveFunctions() []string {
	funcs := make([]string, 0, 6)
	if c.Keyboard {
		funcs = append(funcs, FuncKeyboard)
	}
	if c.Mouse {
		funcs = append(funcs, FuncMouse)
	}
	if c.MassStorage.Enabled {
		funcs = append(funcs, FuncMassStorage)
	}
	if c.RNDIS.Enabled {
		funcs = append(funcs, FuncRNDIS)
	}
	if c.ECM.Enabled {
		funcs = append(funcs, FuncECM)
	}
	if c.ACM.Enabled {
		funcs = append(funcs, FuncACM)
	}
	return funcs
}

// CountEndpoints calculates the total number of hardware endpoints (EP0 + IN + OUT) required by this configuration.
func (c Config) CountEndpoints() int {
	endpoints := 0
	if c.Keyboard {
		endpoints += EndpointsKeyboard
	}
	if c.Mouse {
		endpoints += EndpointsMouse
	}
	if c.MassStorage.Enabled {
		endpoints += EndpointsMassStorage
	}
	if c.RNDIS.Enabled {
		endpoints += EndpointsRNDIS
	}
	if c.ECM.Enabled {
		endpoints += EndpointsECM
	}
	if c.ACM.Enabled {
		endpoints += EndpointsACM
	}
	if endpoints > 0 {
		endpoints += EndpointsEP0
	}
	return endpoints
}
