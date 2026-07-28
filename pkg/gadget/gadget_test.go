package gadget_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/arrase/Raspiducky/pkg/gadget"
)

func createMockUDC(t *testing.T, udcDir string, name string) {
	t.Helper()
	p := filepath.Join(udcDir, name)
	if err := os.MkdirAll(p, 0755); err != nil {
		t.Fatalf("failed creating mock UDC dir: %v", err)
	}
}

func validTestConfig() gadget.Config {
	return gadget.Config{
		VID:          "0x1d6b",
		PID:          "0x0104",
		Manufacturer: "Raspiducky",
		Product:      "USB Gadget",
		Serial:       "123456789",
		Keyboard:     true,
		Mouse:        true,
		MassStorage: gadget.MassStorageConfig{
			Enabled:     true,
			BackingFile: "/tmp/test.img",
			CDROM:       false,
			ReadOnly:    false,
		},
		RNDIS: gadget.EthernetConfig{
			Enabled:  true,
			HostAddr: "00:11:22:33:44:55",
			DevAddr:  "66:77:88:99:aa:bb",
		},
		ECM: gadget.EthernetConfig{
			Enabled:  true,
			HostAddr: "00:11:22:33:44:56",
			DevAddr:  "66:77:88:99:aa:bc",
		},
		ACM: gadget.SerialConfig{
			Enabled: true,
		},
	}
}

func TestConfigValidation(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		cfg := validTestConfig()
		if err := cfg.Validate(); err != nil {
			t.Fatalf("expected valid config, got error: %v", err)
		}
	})

	t.Run("InvalidVIDPID", func(t *testing.T) {
		cfg := validTestConfig()
		cfg.VID = "1d6b" // missing 0x prefix
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for invalid VID")
		}

		cfg = validTestConfig()
		cfg.PID = "invalid"
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for invalid PID")
		}
	})

	t.Run("InvalidMAC", func(t *testing.T) {
		cfg := validTestConfig()
		cfg.RNDIS.HostAddr = "invalid-mac"
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for invalid MAC address")
		}
	})

	t.Run("NoFunctionsEnabled", func(t *testing.T) {
		cfg := validTestConfig()
		cfg.Keyboard = false
		cfg.Mouse = false
		cfg.MassStorage.Enabled = false
		cfg.RNDIS.Enabled = false
		cfg.ECM.Enabled = false
		cfg.ACM.Enabled = false

		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error when no functions are enabled")
		}
	})
}

func TestGadgetDeployAndDestroy(t *testing.T) {
	baseDir := t.TempDir()
	udcDir := t.TempDir()
	createMockUDC(t, udcDir, "20980000.usb")

	gm := gadget.NewGadgetManager(
		gadget.WithBaseDir(baseDir),
		gadget.WithGadgetName("raspiducky"),
		gadget.WithUDCDir(udcDir),
	)

	ctx := context.Background()
	cfg := validTestConfig()

	// Deploy
	if err := gm.Deploy(ctx, cfg); err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	gadgetPath := gm.GadgetPath()

	// Verify base attributes
	vidBytes, err := os.ReadFile(filepath.Join(gadgetPath, "idVendor"))
	if err != nil || string(vidBytes) != "0x1d6b" {
		t.Fatalf("unexpected idVendor content: %q, err: %v", string(vidBytes), err)
	}

	// Verify UDC binding
	udcBytes, err := os.ReadFile(filepath.Join(gadgetPath, "UDC"))
	if err != nil || string(udcBytes) != "20980000.usb" {
		t.Fatalf("unexpected UDC content: %q, err: %v", string(udcBytes), err)
	}

	// Verify Keyboard setup
	kbdReport, err := os.ReadFile(filepath.Join(gadgetPath, "functions", "hid.usb0", "report_desc"))
	if err != nil || !bytes.Equal(kbdReport, gadget.KeyboardReportDesc) {
		t.Fatalf("invalid keyboard report desc")
	}

	// Verify Mouse setup
	mouseReport, err := os.ReadFile(filepath.Join(gadgetPath, "functions", "hid.usb1", "report_desc"))
	if err != nil || !bytes.Equal(mouseReport, gadget.MouseReportDesc) {
		t.Fatalf("invalid mouse report desc")
	}

	// Verify Mass storage setup
	fileBytes, err := os.ReadFile(filepath.Join(gadgetPath, "functions", "mass_storage.usb0", "lun.0", "file"))
	if err != nil || string(fileBytes) != "/tmp/test.img" {
		t.Fatalf("unexpected backing file: %q", string(fileBytes))
	}

	// Verify symlinks in configs/c.1
	cfg1 := filepath.Join(gadgetPath, "configs", "c.1")
	symlinks := []string{"hid.usb0", "hid.usb1", "mass_storage.usb0", "rndis.usb0", "ecm.usb0", "acm.usb0"}
	for _, link := range symlinks {
		p := filepath.Join(cfg1, link)
		fi, err := os.Lstat(p)
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("expected symlink at %s", p)
		}
	}

	// Test updating mass storage backing file dynamically
	if err := gm.SetMassStorageFile(ctx, "/tmp/new.iso", true); err != nil {
		t.Fatalf("SetMassStorageFile failed: %v", err)
	}
	newFileBytes, err := os.ReadFile(filepath.Join(gadgetPath, "functions", "mass_storage.usb0", "lun.0", "file"))
	if err != nil || string(newFileBytes) != "/tmp/new.iso" {
		t.Fatalf("unexpected updated backing file: %q", string(newFileBytes))
	}

	// Destroy
	if err := gm.DestroyGadget(ctx); err != nil {
		t.Fatalf("DestroyGadget failed: %v", err)
	}

	// Verify directory is completely removed
	if _, err := os.Stat(gadgetPath); !os.IsNotExist(err) {
		t.Fatalf("gadget path %s still exists after DestroyGadget", gadgetPath)
	}

	_, deployed := gm.State()
	if deployed {
		t.Fatalf("expected deployed state to be false")
	}
}

func TestContextCancellation(t *testing.T) {
	baseDir := t.TempDir()
	udcDir := t.TempDir()

	gm := gadget.NewGadgetManager(
		gadget.WithBaseDir(baseDir),
		gadget.WithUDCDir(udcDir),
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := validTestConfig()
	if err := gm.Deploy(ctx, cfg); err == nil {
		t.Fatal("expected error on cancelled context deploy")
	}
}
