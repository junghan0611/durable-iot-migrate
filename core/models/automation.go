package models

import "encoding/json"

// Automation represents a platform-agnostic automation rule.
// Known as: recipe (Tuya), automation (Home Assistant),
// rule chain (ThingsBoard), scene/routine (various).
type Automation struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Enabled    bool            `json:"enabled"`
	Triggers   []Trigger       `json:"triggers"`
	Conditions []Condition     `json:"conditions"`
	Actions    []Action        `json:"actions"`
	DeviceRefs []string        `json:"device_refs"` // Device IDs this automation references
	SourceMeta json.RawMessage `json:"source_meta"` // Raw data from source platform
}

// Trigger defines what starts an automation.
type Trigger struct {
	Type     string         `json:"type"`     // "device_state", "schedule", "sun", "webhook", etc.
	DeviceID string         `json:"device_id,omitempty"`
	Config   map[string]any `json:"config"`
}

// Condition defines a guard that must be true for the automation to proceed.
type Condition struct {
	Type     string         `json:"type"`     // "device_state", "time", "numeric", etc.
	DeviceID string         `json:"device_id,omitempty"`
	Config   map[string]any `json:"config"`
}

// Action defines what happens when the automation fires.
type Action struct {
	Type     string         `json:"type"`     // "device_command", "notify", "delay", "scene", etc.
	DeviceID string         `json:"device_id,omitempty"`
	Config   map[string]any `json:"config"`
}
