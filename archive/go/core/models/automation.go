package models

import (
	"encoding/json"

	"github.com/junghan0611/durable-iot-migrate/core/expr"
)

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
	// Expr is the structured expression tree (optional).
	// When present, it is the canonical semantic representation.
	// Config is kept for backward compatibility and platform-specific hints.
	Expr *expr.Expr `json:"expr,omitempty"`
}

// Condition defines a guard that must be true for the automation to proceed.
type Condition struct {
	Type     string         `json:"type"`     // "device_state", "time", "numeric", etc.
	DeviceID string         `json:"device_id,omitempty"`
	Config   map[string]any `json:"config"`
	// Expr is the structured expression tree (optional).
	Expr *expr.Expr `json:"expr,omitempty"`
}

// Action defines what happens when the automation fires.
type Action struct {
	Type     string         `json:"type"`     // "device_command", "notify", "delay", "scene", etc.
	DeviceID string         `json:"device_id,omitempty"`
	Config   map[string]any `json:"config"`
	// Expr is the structured expression tree (optional).
	Expr *expr.Expr `json:"expr,omitempty"`
}
