// Package models defines platform-agnostic data types for IoT migration.
package models

import "encoding/json"

// Device represents a platform-agnostic IoT device.
// Every IoT platform has devices — this is the common denominator.
type Device struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Category   string            `json:"category"`    // "light", "sensor", "lock", "switch", etc.
	Protocol   string            `json:"protocol"`    // "zigbee", "wifi", "thread", "matter", "ble"
	Online     bool              `json:"online"`
	Properties map[string]any    `json:"properties"`  // Platform-specific properties
	SourceMeta json.RawMessage   `json:"source_meta"` // Raw data from source platform
}

// MigrationStatus tracks the state of a single device migration.
type MigrationStatus struct {
	DeviceID    string `json:"device_id"`
	DeviceName  string `json:"device_name"`
	Phase       string `json:"phase"`       // "unbind", "bind", "verify", "complete", "failed", "rolled_back"
	Error       string `json:"error,omitempty"`
	Attempts    int    `json:"attempts"`
	CompletedAt string `json:"completed_at,omitempty"`
}
