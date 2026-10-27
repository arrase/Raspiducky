// Package api provides the HTTP REST API, WebSocket hub, and embedded web dashboard for Raspiducky.
package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/arrase/Raspiducky/pkg/gadget"
	"github.com/arrase/Raspiducky/pkg/hid"
)

const (
	errInvalidJSON      = "Invalid JSON payload"
	maxRequestBodyBytes = 1 << 20 // 1 MB
)

// Server represents the Raspiducky REST API & Embedded Web Dashboard Server.
type Server struct {
	hub           *Hub
	gadgetManager *GadgetManager
	scriptManager *ScriptManager
	runnerEngine  *RunnerEngine
	ledWatcher    *hid.LEDWatcher
	ledUnsub      func()
	mux           *http.ServeMux
}

// ServerOptions allows custom configuration of the API Server.
type ServerOptions struct {
	StorageDir     string
	Keyboard       *hid.Keyboard
	Mouse          *hid.Mouse
	LEDWatcher     *hid.LEDWatcher
	KeyboardLayout string
	TypingDelayMs  int
	GadgetManager  *gadget.GadgetManager
}

// NewServer initializes a new Raspiducky API Server instance.
func NewServer(opts ServerOptions) (*Server, error) {
	sm, err := NewScriptManager(opts.StorageDir)
	if err != nil {
		return nil, err
	}

	hub := NewHub()
	go hub.Run()

	runner := NewRunnerEngine(hub, opts.Keyboard, opts.Mouse, opts.LEDWatcher)
	gm := NewGadgetManager(hub, runner, opts)

	s := &Server{
		hub:           hub,
		gadgetManager: gm,
		scriptManager: sm,
		runnerEngine:  runner,
		ledWatcher:    opts.LEDWatcher,
		mux:           http.NewServeMux(),
	}

	if opts.LEDWatcher != nil {
		s.listenLEDState()
	}

	s.routes()
	return s, nil
}

// Close stops active jobs, unsubscribes watchers, and shuts down the WebSocket hub.
func (s *Server) Close() {
	_ = s.runnerEngine.Stop()
	if s.ledUnsub != nil {
		s.ledUnsub()
	}
	s.hub.Close()
}

// Handler returns the HTTP handler for the server.
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/gadget", s.handleGetGadget)
	s.mux.HandleFunc("POST /api/gadget", s.handlePostGadget)

	s.mux.HandleFunc("GET /api/scripts", s.handleGetScripts)
	s.mux.HandleFunc("POST /api/scripts", s.handlePostScripts)
	s.mux.HandleFunc("DELETE /api/scripts/{name}", s.handleDeleteScript)

	s.mux.HandleFunc("GET /api/job", s.handleGetJob)
	s.mux.HandleFunc("POST /api/run", s.handleRunScript)
	s.mux.HandleFunc("POST /api/stop", s.handleStopScript)

	s.mux.HandleFunc("GET /api/ws", s.handleWS)

	s.mux.Handle("/", http.FileServer(WebFS()))
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.hub.Upgrade(w, r)
	if err != nil {
		return
	}

	if s.ledWatcher != nil {
		_ = conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
		_ = conn.WriteJSON(WSMessage{
			Type:    "led_state",
			Payload: s.ledWatcher.GetState(),
		})
	}

	_ = conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
	_ = conn.WriteJSON(WSMessage{
		Type:    "gadget_status",
		Payload: s.gadgetManager.GetStatus(),
	})

	if job := s.runnerEngine.GetActiveJob(); job != nil {
		_ = conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
		_ = conn.WriteJSON(WSMessage{
			Type:    "job_status",
			Payload: job,
		})
	}

	s.hub.Register(conn)
}

func (s *Server) listenLEDState() {
	ledChan, unsub := s.ledWatcher.Subscribe()
	s.ledUnsub = unsub
	go func() {
		for state := range ledChan {
			s.hub.Broadcast(WSMessage{
				Type:    "led_state",
				Payload: state,
			})
		}
	}()
}

// REST Handlers
func (s *Server) handleGetGadget(w http.ResponseWriter, _ *http.Request) {
	status := s.gadgetManager.GetStatus()
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handlePostGadget(w http.ResponseWriter, r *http.Request) {
	var cfg GadgetConfig
	if err := decodeJSON(w, r, &cfg); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidJSON)
		return
	}

	updated, err := s.gadgetManager.UpdateConfig(cfg)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleGetScripts(w http.ResponseWriter, _ *http.Request) {
	scripts, err := s.scriptManager.ListScripts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, scripts)
}

func (s *Server) handlePostScripts(w http.ResponseWriter, r *http.Request) {
	var script Script
	if err := decodeJSON(w, r, &script); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidJSON)
		return
	}

	savedName, err := s.scriptManager.SaveScript(script)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"message": "Script saved successfully",
		"name":    savedName,
	})
}

func (s *Server) handleDeleteScript(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "Script name parameter required")
		return
	}

	if err := s.scriptManager.DeleteScript(name); err != nil {
		switch {
		case errors.Is(err, ErrInvalidScriptName):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrScriptNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetJob(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.runnerEngine.GetActiveJob())
}

func (s *Server) handleRunScript(w http.ResponseWriter, r *http.Request) {
	var req RunRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidJSON)
		return
	}

	job, err := s.runnerEngine.Run(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Script execution triggered",
		"jobId":   job.ID,
		"status":  job.Status,
	})
}

func (s *Server) handleStopScript(w http.ResponseWriter, _ *http.Request) {
	if err := s.runnerEngine.Stop(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Script execution job stopped",
	})
}

// Helpers
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	return json.NewDecoder(r.Body).Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[API] Failed to encode JSON response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
