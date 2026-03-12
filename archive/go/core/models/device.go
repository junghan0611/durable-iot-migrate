// Package models defines platform-agnostic data types for IoT migration.
package models

import "encoding/json"

// SafetyClass classifies devices by the consequence of migration failure.
// A baby camera going offline is not the same as a hallway light.
type SafetyClass string

const (
	// SafetyCritical — failure can affect physical safety.
	// Examples: camera (child monitoring), door lock, smoke detector, CO sensor.
	// Policy: extra verification, zero-tolerance rollback, human approval gate.
	SafetyCritical SafetyClass = "critical"

	// SafetyImportant — failure causes significant inconvenience but not danger.
	// Examples: thermostat, alarm system, garage door.
	// Policy: extra verification, auto-rollback on failure.
	SafetyImportant SafetyClass = "important"

	// SafetyNormal — failure is inconvenient but not harmful.
	// Examples: light, plug, fan, curtain.
	// Policy: standard migration, batch rollback on threshold.
	SafetyNormal SafetyClass = "normal"
)

// CategorySafetyClass returns the default safety class for a device category.
// This is the baseline — operators can override per-device.
func CategorySafetyClass(category string) SafetyClass {
	switch category {
	case "camera", "lock", "smoke_detector", "co_detector", "alarm", "doorbell":
		return SafetyCritical
	case "thermostat", "garage", "water_valve", "gas_valve":
		return SafetyImportant
	default:
		return SafetyNormal
	}
}

// Device represents a platform-agnostic IoT device.
// Every IoT platform has devices — this is the common denominator.
type Device struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Category    string            `json:"category"`     // "light", "sensor", "lock", "switch", etc.
	Protocol    string            `json:"protocol"`     // "zigbee", "wifi", "thread", "matter", "ble"
	Online      bool              `json:"online"`
	AccountID   string            `json:"account_id"`   // Owner account
	SafetyClass SafetyClass       `json:"safety_class"` // Derived from category, can be overridden
	Properties  map[string]any    `json:"properties"`   // Platform-specific properties
	SourceMeta  json.RawMessage   `json:"source_meta"`  // Raw data from source platform
}

// IsCritical returns true if this device's failure could affect physical safety.
func (d *Device) IsCritical() bool {
	return d.SafetyClass == SafetyCritical
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
