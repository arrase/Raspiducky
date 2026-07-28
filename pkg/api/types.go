package api

import (
	"time"
)

// GadgetConfig represents the configurable USB parameters and peripheral profiles.
type GadgetConfig struct {
	Keyboard       bool   `json:"keyboard"`
	Mouse          bool   `json:"mouse"`
	Storage        bool   `json:"storage"`
	Ethernet       bool   `json:"ethernet"`
	Serial         bool   `json:"serial"`
	VendorID       string `json:"vendorId"`
	ProductID      string `json:"productId"`
	Manufacturer   string `json:"manufacturer"`
	Product        string `json:"product"`
	SerialNumber   string `json:"serialNumber"`
	StorageSizeMB  int    `json:"storageSizeMb"`
	KeyboardLayout string `json:"keyboardLayout"`
}

// GadgetStatus represents the active status of the USB gadget subsystem.
type GadgetStatus struct {
	Deployed        bool         `json:"deployed"`
	ActiveFunctions []string     `json:"activeFunctions"`
	UDC             string       `json:"udc"`
	Config          GadgetConfig `json:"config"`
}

// Script represents a saved payload script.
type Script struct {
	Name        string    `json:"name"`
	Type        string    `json:"type"` // "duckyscript" or "javascript"
	Content     string    `json:"content"`
	Description string    `json:"description,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// RunRequest payload for starting script execution.
type RunRequest struct {
	Script string `json:"script"`
	Name   string `json:"name,omitempty"`
	Type   string `json:"type"` // "duckyscript" or "javascript"
}

// JobStatus describes an active or past script execution job.
type JobStatus struct {
	ID         string     `json:"id"`
	ScriptName string     `json:"name"`
	Type       string     `json:"type"`
	Status     string     `json:"status"` // "running", "completed", "stopped", "failed"
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	Error      string     `json:"error,omitempty"`
}

// LEDState represents keyboard lock LED indicators.
type LEDState struct {
	NumLock    bool `json:"numLock"`
	CapsLock   bool `json:"capsLock"`
	ScrollLock bool `json:"scrollLock"`
}

// WSMessage format for broadcasting real-time events to connected clients.
type WSMessage struct {
	Type    string      `json:"type"` // "log", "led_state", "gadget_status", "job_status"
	Level   string      `json:"level,omitempty"`
	Source  string      `json:"source,omitempty"`
	Message string      `json:"message,omitempty"`
	Payload interface{} `json:"payload,omitempty"`
}
