package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/arrase/Raspiducky/pkg/api"
	"github.com/arrase/Raspiducky/pkg/hid"
	"github.com/arrase/Raspiducky/pkg/scripting"
)

const version = "v1.0.0-modern"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "daemon", "server":
		if err := runDaemon(os.Args[2:]); err != nil {
			log.Fatalf("Error: %v", err)
		}
	case "run", "exec":
		if err := runScriptCLI(os.Args[2:]); err != nil {
			log.Fatalf("Error: %v", err)
		}
	case "version", "-v", "--version":
		fmt.Printf("Raspiducky %s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		// Default to running daemon if first arg looks like a flag
		if len(command) > 0 && command[0] == '-' {
			if err := runDaemon(os.Args[1:]); err != nil {
				log.Fatalf("Error: %v", err)
			}
		} else {
			fmt.Printf("Unknown command: %s\n\n", command)
			printUsage()
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Println("Raspiducky - Modern USB Gadget & DuckyScript Appliance")
	fmt.Println("Version:", version)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  raspiducky daemon [flags]      Start the background server & web dashboard")
	fmt.Println("  raspiducky run <file> [flags]  Execute a script (DuckyScript .txt or JS .js)")
	fmt.Println("  raspiducky version             Print version information")
	fmt.Println("  raspiducky help                Show this help message")
	fmt.Println()
	fmt.Println("Daemon Flags:")
	fmt.Println("  -port string     Port for Web Dashboard & API (default \":8000\")")
	fmt.Println("  -storage string  Path to persistent storage directory (default \"/var/lib/raspiducky\")")
	fmt.Println("  -layout string   Keyboard layout (US, ES, DE, FR) (default \"US\")")
	fmt.Println("  -delay int       Keystroke typing delay in ms (default 10)")
}

func initHIDDevices(ctx context.Context, layout string, delayMs int) (*hid.Keyboard, *hid.Mouse, *hid.LEDWatcher, error) {
	kbd, err := hid.NewKeyboard("/dev/hidg0", layout)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to initialize keyboard: %w", err)
	}
	kbd.SetTypingSpeed(delayMs, 0)

	mouse := hid.NewMouse("/dev/hidg1")
	ledWatcher := hid.NewLEDWatcher(ctx, "/dev/hidg0")

	return kbd, mouse, ledWatcher, nil
}

func runDaemon(args []string) error {
	fs := flag.NewFlagSet("daemon", flag.ContinueOnError)
	port := fs.String("port", ":8000", "Server port")
	storageDir := fs.String("storage", "/var/lib/raspiducky", "Storage directory")
	layout := fs.String("layout", "US", "Keyboard layout")
	delay := fs.Int("delay", 10, "Keystroke typing delay in ms")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	log.Printf("Starting Raspiducky Daemon %s on %s...", version, *port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kbd, mouse, ledWatcher, err := initHIDDevices(ctx, *layout, *delay)
	if err != nil {
		return err
	}
	defer kbd.Close()
	defer mouse.Close()
	defer ledWatcher.Close()

	server, err := api.NewServer(api.ServerOptions{
		StorageDir: *storageDir,
		Keyboard:   kbd,
		Mouse:      mouse,
		LEDWatcher: ledWatcher,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	httpServer := &http.Server{
		Addr:    *port,
		Handler: server.Handler(),
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	log.Printf("Raspiducky Web Dashboard running at http://localhost%s", *port)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down daemon gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error shutting down HTTP server: %v", err)
	}
	log.Println("Raspiducky Daemon stopped.")
	return nil
}

func runScriptCLI(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	layout := fs.String("layout", "US", "Keyboard layout")
	delay := fs.Int("delay", 10, "Keystroke typing delay in ms")

	var filePath string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		filePath = args[0]
		if err := fs.Parse(args[1:]); err != nil {
			return fmt.Errorf("parsing flags: %w", err)
		}
	} else {
		if err := fs.Parse(args); err != nil {
			return fmt.Errorf("parsing flags: %w", err)
		}
		filePath = fs.Arg(0)
	}
	if filePath == "" {
		return errors.New("usage: raspiducky run <script-file>")
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading script file %s: %w", filePath, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	kbd, mouse, ledWatcher, err := initHIDDevices(ctx, *layout, *delay)
	if err != nil {
		return err
	}
	defer kbd.Close()
	defer mouse.Close()
	defer ledWatcher.Close()
	defer func() { _ = kbd.ReleaseKeys() }()

	engine := scripting.NewScriptEngine(kbd, mouse, ledWatcher)

	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == ".txt" || ext == ".ducky" {
		err = engine.RunDuckyScript(ctx, string(content), os.Stdout)
	} else {
		err = engine.RunJS(ctx, string(content), os.Stdout)
	}
	if err != nil {
		return err
	}

	log.Println("Script execution finished cleanly.")
	return nil
}
