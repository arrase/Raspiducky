package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/arrase/Raspiducky/pkg/hid"
)

// Server represents the Raspiducky REST API & Embedded Web Dashboard Server.
type Server struct {
	hub           *Hub
	gadgetManager *GadgetManager
	scriptManager *ScriptManager
	runnerEngine  *RunnerEngine
	ledWatcher    *hid.LEDWatcher
	mux           *http.ServeMux
}

// ServerOptions allows custom configuration of the API Server.
type ServerOptions struct {
	StorageDir string
	Keyboard   *hid.Keyboard
	Mouse      *hid.Mouse
	LEDWatcher *hid.LEDWatcher
}

// NewServer initializes a new Raspiducky API Server instance.
func NewServer(opts ServerOptions) (*Server, error) {
	hub := NewHub()
	go hub.Run()

	gm := NewGadgetManager(hub, opts.Keyboard, opts.StorageDir)

	sm, err := NewScriptManager(opts.StorageDir)
	if err != nil {
		return nil, err
	}

	runner := NewRunnerEngine(hub, opts.Keyboard, opts.Mouse, opts.LEDWatcher)

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

// Handler returns the HTTP handler for the server.
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	// REST API Endpoints
	s.mux.HandleFunc("GET /api/gadget", s.handleGetGadget)
	s.mux.HandleFunc("POST /api/gadget", s.handlePostGadget)

	s.mux.HandleFunc("GET /api/scripts", s.handleGetScripts)
	s.mux.HandleFunc("POST /api/scripts", s.handlePostScripts)
	s.mux.HandleFunc("DELETE /api/scripts/{name}", s.handleDeleteScript)

	s.mux.HandleFunc("POST /api/run", s.handleRunScript)
	s.mux.HandleFunc("POST /api/stop", s.handleStopScript)

	// WebSocket Endpoint
	s.mux.HandleFunc("GET /api/ws", s.handleWS)

	// Static Web Assets Embedding
	webFS, err := WebFS()
	if err == nil {
		s.mux.Handle("/", http.FileServer(webFS))
	}
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.hub.Upgrade(w, r)
	if err != nil {
		return
	}

	if s.ledWatcher != nil {
		_ = conn.WriteJSON(WSMessage{
			Type:    "led_state",
			Payload: s.ledWatcher.GetState(),
		})
	}
}

func (s *Server) listenLEDState() {
	ledChan, _ := s.ledWatcher.Subscribe()
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
func (s *Server) handleGetGadget(w http.ResponseWriter, r *http.Request) {
	status := s.gadgetManager.GetStatus()
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handlePostGadget(w http.ResponseWriter, r *http.Request) {
	var cfg GadgetConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := s.gadgetManager.UpdateConfig(cfg)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleGetScripts(w http.ResponseWriter, r *http.Request) {
	scripts, err := s.scriptManager.ListScripts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, scripts)
}

func (s *Server) handlePostScripts(w http.ResponseWriter, r *http.Request) {
	var script Script
	if err := json.NewDecoder(r.Body).Decode(&script); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if err := s.scriptManager.SaveScript(script); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"message": "Script saved successfully",
		"name":    script.Name,
	})
}

func (s *Server) handleDeleteScript(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "Script name parameter required")
		return
	}

	if err := s.scriptManager.DeleteScript(name); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRunScript(w http.ResponseWriter, r *http.Request) {
	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON payload")
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

func (s *Server) handleStopScript(w http.ResponseWriter, r *http.Request) {
	if err := s.runnerEngine.Stop(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Script execution job stopped",
	})
}

// Helpers
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
