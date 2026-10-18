package gadget

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func setupMockHardware(t *testing.T, maxEndpoints string) *GadgetManager {
	t.Helper()
	tmpDir := t.TempDir()

	baseDir := filepath.Join(tmpDir, "usb_gadget")
	udcDir := filepath.Join(tmpDir, "udc")
	debugFSDir := filepath.Join(tmpDir, "debugfs")

	if err := os.MkdirAll(udcDir, 0755); err != nil {
		t.Fatalf("failed to create udc dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(udcDir, "dummy.udc"), []byte(""), 0644); err != nil {
		t.Fatalf("failed to create dummy udc: %v", err)
	}

	udcDebugDir := filepath.Join(debugFSDir, "dummy.udc")
	if err := os.MkdirAll(udcDebugDir, 0755); err != nil {
		t.Fatalf("failed to create debugfs dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(udcDebugDir, "hw_params"), []byte("num_dev_ep: "+maxEndpoints+"\n"), 0644); err != nil {
		t.Fatalf("failed to write hw_params: %v", err)
	}

	return NewGadgetManager(
		WithBaseDir(baseDir),
		WithUDCDir(udcDir),
		WithDebugFSDir(debugFSDir),
	)
}

func validTestConfig() Config {
	return Config{
		VID:          "0x1d6b",
		PID:          "0x0104",
		Manufacturer: "Raspiducky Labs",
		Product:      "Raspiducky HID",
		Serial:       "RPD-001",
		Keyboard:     true,
		Mouse:        true,
	}
}

func TestDeploy_FullCompositeAndRedeployTeardown(t *testing.T) {
	gm := setupMockHardware(t, "16")
	ctx := context.Background()

	cfg1 := validTestConfig()
	cfg1.MassStorage = MassStorageConfig{
		Enabled:     true,
		BackingFile: "/tmp/disk.img",
		CDROM:       true,
	}
	cfg1.RNDIS = EthernetConfig{
		Enabled:  true,
		HostAddr: "02:00:00:00:00:01",
		DevAddr:  "02:00:00:00:00:02",
	}
	cfg1.ECM = EthernetConfig{
		Enabled:  true,
		HostAddr: "02:00:00:00:00:03",
		DevAddr:  "02:00:00:00:00:04",
	}
	cfg1.ACM = SerialConfig{Enabled: true}

	if err := gm.Deploy(ctx, cfg1); err != nil {
		t.Fatalf("Deploy(cfg1) failed: %v", err)
	}

	gadgetPath := gm.GadgetPath()

	// Verify RNDIS os_desc/c.1 symlink and Mass Storage lun.0 exist
	osDescLink := filepath.Join(gadgetPath, "os_desc", "c.1")
	if info, err := os.Lstat(osDescLink); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected os_desc/c.1 symlink to exist, err=%v", err)
	}

	lunFile := filepath.Join(gadgetPath, "functions", FuncMassStorage, "lun.0", "file")
	if data, err := os.ReadFile(lunFile); err != nil || string(data) != "/tmp/disk.img" {
		t.Fatalf("expected mass storage lun.0/file='/tmp/disk.img', got %q (err=%v)", string(data), err)
	}

	// Redeploy with a minimal keyboard-only configuration to verify full teardown of RNDIS & Mass Storage
	cfg2 := validTestConfig()
	cfg2.Mouse = false
	if err := gm.Deploy(ctx, cfg2); err != nil {
		t.Fatalf("Redeploy(cfg2) failed: %v", err)
	}

	if _, err := os.Lstat(osDescLink); !os.IsNotExist(err) {
		t.Errorf("expected os_desc/c.1 to be removed after redeploy, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(gadgetPath, "functions", "rndis.usb0")); !os.IsNotExist(err) {
		t.Errorf("expected rndis.usb0 function directory to be removed after redeploy")
	}
	if _, err := os.Stat(filepath.Join(gadgetPath, "functions", FuncMassStorage)); !os.IsNotExist(err) {
		t.Errorf("expected mass_storage.usb0 function directory to be removed after redeploy")
	}
	if _, err := os.Stat(filepath.Join(gadgetPath, "functions", "hid.usb0")); err != nil {
		t.Errorf("expected hid.usb0 function directory to exist after redeploy: %v", err)
	}
}

func TestDestroyGadgetUnlocked_TeardownOrder(t *testing.T) {
	gm := setupMockHardware(t, "16")
	ctx := context.Background()

	cfg := validTestConfig()
	cfg.RNDIS = EthernetConfig{
		Enabled:  true,
		HostAddr: "02:00:00:00:00:01",
		DevAddr:  "02:00:00:00:00:02",
	}

	if err := gm.Deploy(ctx, cfg); err != nil {
		t.Fatalf("initial Deploy failed: %v", err)
	}

	gadgetPath := gm.GadgetPath()
	configsParent := filepath.Join(gadgetPath, "configs")
	cfgDir := filepath.Join(configsParent, "c.1")
	osDescLink := filepath.Join(gadgetPath, "os_desc", "c.1")

	// Make configs/ read-only so removing configs/c.1 (Step 4) fails.
	// This allows us to verify that Step 1 (unbind UDC), Step 2 (unlink os_desc/c.1),
	// and Step 3 (unlink function symlinks in configs/c.1) all happen BEFORE configs/c.1 is removed.
	if err := os.Chmod(configsParent, 0555); err != nil {
		t.Fatalf("chmod configs parent failed: %v", err)
	}
	defer func() {
		_ = os.Chmod(configsParent, 0755)
	}()

	err := gm.destroyGadgetUnlocked(gadgetPath)
	if err == nil {
		t.Fatal("expected error when configs/c.1 cannot be removed")
	}

	// Verify UDC was unbound in Step 1
	if udcData, readErr := os.ReadFile(filepath.Join(gadgetPath, "UDC")); readErr != nil || len(udcData) != 0 {
		t.Errorf("expected UDC to be unbound (empty) before configs/c.1 removal, got %q (err=%v)", string(udcData), readErr)
	}

	// Verify os_desc/c.1 was already removed in Step 2 before configs/c.1 removal failed in Step 4
	if _, statErr := os.Lstat(osDescLink); !os.IsNotExist(statErr) {
		t.Errorf("expected os_desc/c.1 to be removed before configs/c.1, but Lstat returned: %v", statErr)
	}

	// Verify function symlinks inside configs/c.1 were already removed in Step 3
	if _, statErr := os.Lstat(filepath.Join(cfgDir, "rndis.usb0")); !os.IsNotExist(statErr) {
		t.Errorf("expected configs/c.1/rndis.usb0 symlink to be removed before configs/c.1")
	}

	// Verify configs/c.1 itself still exists (since Step 4 was blocked by chmod 0555)
	if _, statErr := os.Stat(cfgDir); statErr != nil {
		t.Errorf("expected configs/c.1 to still exist when Step 4 failed: %v", statErr)
	}
}

func TestDeploy_EndpointLimitExceeded(t *testing.T) {
	gm := setupMockHardware(t, "2")
	ctx := context.Background()

	cfg := validTestConfig()
	cfg.RNDIS = EthernetConfig{
		Enabled:  true,
		HostAddr: "02:00:00:00:00:01",
		DevAddr:  "02:00:00:00:00:02",
	}

	if err := gm.Deploy(ctx, cfg); err == nil {
		t.Fatal("expected error when configuration exceeds hardware IN endpoint limit")
	}
}

func TestConfig_Validate(t *testing.T) {
	cfg := validTestConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config, got: %v", err)
	}

	noFuncs := validTestConfig()
	noFuncs.Keyboard = false
	noFuncs.Mouse = false
	if err := noFuncs.Validate(); err != ErrNoFunctionsEnabled {
		t.Errorf("expected ErrNoFunctionsEnabled, got: %v", err)
	}

	badVID := validTestConfig()
	badVID.VID = "1d6b"
	if err := badVID.Validate(); err == nil {
		t.Error("expected error for invalid VID format")
	}
}
