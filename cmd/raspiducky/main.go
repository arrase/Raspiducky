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
}

func runDaemon(args []string) error {
	fs := flag.NewFlagSet("daemon", flag.ExitOnError)
	port := fs.String("port", ":8000", "Server port")
	storageDir := fs.String("storage", "/var/lib/raspiducky", "Storage directory")
	layout := fs.String("layout", "US", "Keyboard layout")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	log.Printf("Starting Raspiducky Daemon %s on %s...", version, *port)

	kbd, err := hid.NewKeyboard("/dev/hidg0", *layout)
	if err != nil {
		log.Printf("Warning: failed to initialize keyboard: %v", err)
	}

	mouse, err := hid.NewMouse("/dev/hidg1")
	if err != nil {
		log.Printf("Warning: failed to initialize mouse: %v", err)
	}

	ledWatcher := hid.NewLEDWatcher(context.Background(), "/dev/hidg0")

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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Error shutting down HTTP server: %v", err)
	}
	log.Println("Raspiducky Daemon stopped.")
	return nil
}

func runScriptCLI(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: raspiducky run <script-file>")
	}

	filePath := args[0]
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading script file %s: %w", filePath, err)
	}

	scriptType := "js"
	if filepath.Ext(filePath) == ".txt" {
		scriptType = "ducky"
	}

	// Parse CLI flags for layout if provided
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	layout := fs.String("layout", "US", "Keyboard layout")
	if err := fs.Parse(args[1:]); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	kbd, err := hid.NewKeyboard("/dev/hidg0", *layout)
	if err != nil {
		log.Printf("Warning: failed to initialize keyboard: %v", err)
	}

	mouse, err := hid.NewMouse("/dev/hidg1")
	if err != nil {
		log.Printf("Warning: failed to initialize mouse: %v", err)
	}

	ledWatcher := hid.NewLEDWatcher(context.Background(), "/dev/hidg0")

	engine := scripting.NewScriptEngine(kbd, mouse, ledWatcher)
	runner := scripting.NewRunner(engine)

	job, err := runner.SubmitJob(scriptType, string(content))
	if err != nil {
		return fmt.Errorf("starting script: %w", err)
	}

	log.Printf("Started script job %s (%s)", job.ID, job.ScriptType)

	// Wait for job completion
	for {
		time.Sleep(200 * time.Millisecond)
		j, ok := runner.GetJob(job.ID)
		if !ok || j.Status == scripting.StatusCompleted || j.Status == scripting.StatusFailed || j.Status == scripting.StatusCancelled {
			if j != nil && j.Error != "" {
				return fmt.Errorf("job error: %s", j.Error)
			}
			log.Println("Script execution finished cleanly.")
			break
		}
	}
	return nil
}
