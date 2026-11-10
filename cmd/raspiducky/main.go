// Package main provides the Raspiducky daemon and CLI entrypoint.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
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

var version = "dev"

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
		if strings.HasPrefix(command, "-") {
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
	fmt.Println("  raspiducky run <file> [flags]  Execute a script (DuckyScript .ducky/.txt or JS .js)")
	fmt.Println("  raspiducky version             Print version information")
	fmt.Println("  raspiducky help                Show this help message")
	fmt.Println()
	fmt.Println("Daemon Flags:")
	fmt.Println("  -port string     Port for Web Dashboard & API (default \":8000\")")
	fmt.Println("  -storage string  Path to persistent storage directory (default \"/var/lib/raspiducky\")")
	fmt.Println("  -layout string   Override keyboard layout (US, ES, DE, FR)")
	fmt.Println("  -delay int       Override keystroke typing delay in ms (default -1 uses saved/10ms)")
}

func initHIDDevices(ctx context.Context, layout string, delayMs int) (*hid.Keyboard, *hid.Mouse, *hid.LEDWatcher, error) {
	if layout == "" {
		layout = "US"
	}
	if delayMs < 0 {
		delayMs = 10
	}

	kbd, err := hid.NewKeyboard("/dev/hidg0", layout)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to initialize keyboard: %w", err)
	}
	kbd.SetTypingSpeed(delayMs, 0)

	mouse := hid.NewMouse("/dev/hidg1")
	ledWatcher := hid.NewLEDWatcher(ctx, "/dev/hidg0")

	return kbd, mouse, ledWatcher, nil
}

func resolveDelayOverride(delayFlag int) *int {
	if delayFlag < 0 {
		return nil
	}
	v := delayFlag
	return &v
}

func runDaemon(args []string) error {
	fs := flag.NewFlagSet("daemon", flag.ContinueOnError)
	port := fs.String("port", ":8000", "Server port")
	storageDir := fs.String("storage", "/var/lib/raspiducky", "Storage directory")
	layout := fs.String("layout", "", "Keyboard layout override (US, ES, DE, FR)")
	delay := fs.Int("delay", -1, "Keystroke typing delay override in ms")
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
	defer func() { _ = kbd.Close() }()
	defer func() { _ = mouse.Close() }()
	defer func() { _ = ledWatcher.Close() }()

	server, err := api.NewServer(api.ServerOptions{
		StorageDir:     *storageDir,
		Keyboard:       kbd,
		Mouse:          mouse,
		LEDWatcher:     ledWatcher,
		KeyboardLayout: *layout,
		TypingDelayMs:  resolveDelayOverride(*delay),
	})
	if err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}
	defer server.Close()

	httpServer := &http.Server{
		Addr:              *port,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if serveErr := httpServer.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
		}
	}()

	displayAddr := *port
	if strings.HasPrefix(displayAddr, ":") {
		displayAddr = "localhost" + displayAddr
	}
	log.Printf("Raspiducky Web Dashboard running at http://%s", displayAddr)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)

	select {
	case serveErr := <-errCh:
		return fmt.Errorf("HTTP server error: %w", serveErr)
	case <-stop:
	}

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
	layout := fs.String("layout", "", "Keyboard layout override (US, ES, DE, FR)")
	delay := fs.Int("delay", -1, "Keystroke typing delay override in ms")
	serverURL := fs.String("server", "http://127.0.0.1:8000", "Daemon API URL")

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

	delayOverride := resolveDelayOverride(*delay)
	if handled, err := runViaDaemon(ctx, strings.TrimRight(*serverURL, "/"), filepath.Base(filePath), string(content), *layout, delayOverride); handled {
		if err != nil {
			return err
		}
		log.Println("Script execution finished cleanly via daemon.")
		return nil
	}

	kbd, mouse, ledWatcher, err := initHIDDevices(ctx, *layout, *delay)
	if err != nil {
		return err
	}
	defer func() { _ = kbd.Close() }()
	defer func() { _ = mouse.Close() }()
	defer func() { _ = ledWatcher.Close() }()

	stopInterrupt := context.AfterFunc(ctx, func() {
		kbd.Interrupt()
		mouse.Interrupt()
	})
	defer stopInterrupt()

	engine := scripting.NewScriptEngine(kbd, mouse, ledWatcher)
	if err := engine.Run(ctx, "", filePath, string(content), os.Stdout); err != nil {
		return err
	}

	log.Println("Script execution finished cleanly.")
	return nil
}

func stopDaemonJob(client *http.Client, baseURL string) {
	stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stopReq, err := http.NewRequestWithContext(stopCtx, http.MethodPost, baseURL+"/api/stop", bytes.NewReader([]byte("{}")))
	if err != nil {
		return
	}
	stopReq.Header.Set("Content-Type", "application/json")
	if resp, stopErr := client.Do(stopReq); stopErr == nil {
		_ = resp.Body.Close()
	}
}

func isDaemonUnreachable(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Op == "dial" {
		return true
	}
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr)
}

func runViaDaemon(ctx context.Context, baseURL, name, content, layout string, delayMs *int) (bool, error) {
	if baseURL == "" {
		return false, nil
	}

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{Timeout: 500 * time.Millisecond}).DialContext,
		},
		Timeout: 5 * time.Second,
	}

	probeReq, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/job", nil)
	if err != nil {
		return false, nil
	}
	probeResp, err := client.Do(probeReq)
	if err != nil {
		if ctx.Err() != nil {
			return true, ctx.Err()
		}
		if isDaemonUnreachable(err) {
			return false, nil
		}
		return true, fmt.Errorf("contacting daemon at %s: %w", baseURL, err)
	}
	_ = probeResp.Body.Close()
	if probeResp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if probeResp.StatusCode != http.StatusOK {
		return true, fmt.Errorf("daemon health check returned status %d", probeResp.StatusCode)
	}

	reqBody, err := json.Marshal(api.RunRequest{
		Script:        content,
		Name:          name,
		Type:          scripting.DetectScriptType("", name),
		TypingDelayMs: delayMs,
		Layout:        layout,
	})
	if err != nil {
		return true, err
	}

	runReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/run", bytes.NewReader(reqBody))
	if err != nil {
		return true, err
	}
	runReq.Header.Set("Content-Type", "application/json")

	runResp, err := client.Do(runReq)
	if err != nil {
		return true, err
	}
	defer func() { _ = runResp.Body.Close() }()

	if runResp.StatusCode != http.StatusOK {
		var errPayload map[string]string
		_ = json.NewDecoder(runResp.Body).Decode(&errPayload)
		if msg := errPayload["error"]; msg != "" {
			return true, errors.New(msg)
		}
		return true, fmt.Errorf("daemon returned status %d", runResp.StatusCode)
	}

	var startPayload struct {
		JobID string `json:"jobId"`
	}
	if err := json.NewDecoder(runResp.Body).Decode(&startPayload); err != nil {
		return true, err
	}

	return true, pollDaemonJob(ctx, client, baseURL, startPayload.JobID)
}

func pollDaemonJob(ctx context.Context, client *http.Client, baseURL, jobID string) error {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			stopDaemonJob(client, baseURL)
			return ctx.Err()

		case <-ticker.C:
			done, err := checkDaemonJobStatus(ctx, client, baseURL, jobID)
			if err != nil {
				return err
			}
			if done {
				return nil
			}
		}
	}
}

func checkDaemonJobStatus(ctx context.Context, client *http.Client, baseURL, jobID string) (bool, error) {
	statusReq, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/job", nil)
	if err != nil {
		if ctx.Err() != nil {
			stopDaemonJob(client, baseURL)
			return false, ctx.Err()
		}
		return false, err
	}
	statusResp, err := client.Do(statusReq)
	if err != nil {
		if ctx.Err() != nil {
			stopDaemonJob(client, baseURL)
			return false, ctx.Err()
		}
		return false, err
	}
	defer func() { _ = statusResp.Body.Close() }()

	if statusResp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("daemon job status returned %d", statusResp.StatusCode)
	}

	var job *api.JobStatus
	if err := json.NewDecoder(statusResp.Body).Decode(&job); err != nil {
		return false, fmt.Errorf("decoding daemon job status: %w", err)
	}
	if job == nil || job.ID != jobID {
		return false, errors.New("daemon job was superseded or reset")
	}

	switch job.Status {
	case api.JobStatusCompleted:
		return true, nil
	case api.JobStatusFailed:
		if job.Error != "" {
			return false, errors.New(job.Error)
		}
		return false, errors.New("script execution failed on daemon")
	case api.JobStatusStopped:
		return false, context.Canceled
	default:
		return false, nil
	}
}
