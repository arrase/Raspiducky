package api

import (
	"time"
)

// GadgetConfig represents the configurable USB parameters and peripheral profiles.
type GadgetConfig struct {
	Keyboard        bool   `json:"keyboard"`
	Mouse           bool   `json:"mouse"`
	Storage         bool   `json:"storage"`
	StorageCDROM    bool   `json:"storageCdrom"`
	StorageReadOnly bool   `json:"storageReadOnly"`
	Ethernet        bool   `json:"ethernet,omitempty"`
	RNDIS           bool   `json:"rndis"`
	ECM             bool   `json:"ecm"`
	Serial          bool   `json:"serial"`
	VendorID        string `json:"vendorId"`
	ProductID       string `json:"productId"`
	Manufacturer    string `json:"manufacturer"`
	Product         string `json:"product"`
	SerialNumber    string `json:"serialNumber"`
	StorageSizeMB   int    `json:"storageSizeMb"`
	KeyboardLayout  string `json:"keyboardLayout"`
	TypingDelayMs   int    `json:"typingDelayMs"`
	RNDISHostAddr   string `json:"rndisHostAddr,omitempty"`
	RNDISDevAddr    string `json:"rndisDevAddr,omitempty"`
	ECMHostAddr     string `json:"ecmHostAddr,omitempty"`
	ECMDevAddr      string `json:"ecmDevAddr,omitempty"`
}

// GadgetStatus represents the active status of the USB gadget subsystem.
type GadgetStatus struct {
	Deployed        bool           `json:"deployed"`
	ActiveFunctions []string       `json:"activeFunctions"`
	UDC             string         `json:"udc"`
	MaxEndpoints    int            `json:"maxEndpoints"`
	UsedEndpoints   int            `json:"usedEndpoints"`
	EndpointCosts   map[string]int `json:"endpointCosts"`
	Config          GadgetConfig   `json:"config"`
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
	Script        string `json:"script"`
	Name          string `json:"name,omitempty"`
	Type          string `json:"type"` // "duckyscript" or "javascript"
	TypingDelayMs int    `json:"typingDelayMs,omitempty"`
	Layout        string `json:"layout,omitempty"`
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

// WSMessage format for broadcasting real-time events to connected clients.
type WSMessage struct {
	Type    string `json:"type"` // "log", "led_state", "gadget_status", "job_status"
	Level   string `json:"level,omitempty"`
	Source  string `json:"source,omitempty"`
	Message string `json:"message,omitempty"`
	Payload any    `json:"payload,omitempty"`
}
