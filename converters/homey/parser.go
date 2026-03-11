// Package homey implements the converter for Homey Flow JSON.
//
// Homey uses a "card" system: When → And → Then.
// Flows are exported via Homey Web API: GET /api/flow/flow
//
// Flow JSON structure (from Homey Pro backup):
//
//	{
//	  "id": "abc123",
//	  "name": "Motion Light",
//	  "enabled": true,
//	  "trigger": {
//	    "uri": "homey:device:sensor01",
//	    "id": "alarm_motion_true",
//	    "args": {"device": {"id": "sensor01", "name": "PIR Sensor"}}
//	  },
//	  "conditions": [
//	    {
//	      "uri": "homey:device:light01",
//	      "id": "onoff_false",
//	      "args": {"device": {"id": "light01"}},
//	      "inverted": false
//	    }
//	  ],
//	  "actions": [
//	    {
//	      "uri": "homey:device:light01",
//	      "id": "on",
//	      "args": {"device": {"id": "light01"}}
//	    },
//	    {
//	      "uri": "homey:manager:flow",
//	      "id": "delay",
//	      "args": {"delay": 300}
//	    }
//	  ]
//	}
package homey

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/junghan0611/durable-iot-migrate/core/converter"
	"github.com/junghan0611/durable-iot-migrate/core/models"
)

// Parser reads Homey Flow JSON (from Web API or backup).
type Parser struct{}

func (p *Parser) Platform() string { return "homey" }

// Homey flow JSON structure
type homeyFlow struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	Enabled    bool             `json:"enabled"`
	Trigger    *homeyCard       `json:"trigger"`
	Conditions []homeyCard      `json:"conditions"`
	Actions    []homeyCard      `json:"actions"`
}

type homeyCard struct {
	URI      string         `json:"uri"`      // "homey:device:xxx", "homey:manager:flow", etc.
	ID       string         `json:"id"`       // Card ID: "alarm_motion_true", "on", "delay"
	Args     map[string]any `json:"args"`     // Arguments (device ref, values, etc.)
	Inverted bool           `json:"inverted"` // For conditions: negate the check
}

func (p *Parser) ParseBytes(data []byte) ([]models.Automation, error) {
	var flows []homeyFlow
	if err := json.Unmarshal(data, &flows); err != nil {
		var single homeyFlow
		if err2 := json.Unmarshal(data, &single); err2 != nil {
			return nil, fmt.Errorf("parse Homey JSON: %w", err)
		}
		flows = []homeyFlow{single}
	}

	result := make([]models.Automation, 0, len(flows))
	for _, flow := range flows {
		auto := models.Automation{
			ID:      flow.ID,
			Name:    flow.Name,
			Enabled: flow.Enabled,
		}

		// Trigger (When...)
		if flow.Trigger != nil {
			trigger := convertHomeyTrigger(flow.Trigger)
			auto.Triggers = append(auto.Triggers, trigger)
			if trigger.DeviceID != "" {
				auto.DeviceRefs = appendUnique(auto.DeviceRefs, trigger.DeviceID)
			}
		}

		// Conditions (And...)
		for _, c := range flow.Conditions {
			cond := convertHomeyCondition(&c)
			auto.Conditions = append(auto.Conditions, cond)
			if cond.DeviceID != "" {
				auto.DeviceRefs = appendUnique(auto.DeviceRefs, cond.DeviceID)
			}
		}

		// Actions (Then...)
		for _, a := range flow.Actions {
			action := convertHomeyAction(&a)
			auto.Actions = append(auto.Actions, action)
			if action.DeviceID != "" {
				auto.DeviceRefs = appendUnique(auto.DeviceRefs, action.DeviceID)
			}
		}

		auto.SourceMeta, _ = json.Marshal(flow)
		result = append(result, auto)
	}

	return result, nil
}

// URI format: "homey:device:DEVICE_ID" or "homey:manager:MANAGER" or "homey:app:APP_ID"
func extractDeviceID(uri string) string {
	parts := strings.SplitN(uri, ":", 3)
	if len(parts) == 3 && parts[1] == "device" {
		return parts[2]
	}
	return ""
}

func extractManager(uri string) string {
	parts := strings.SplitN(uri, ":", 3)
	if len(parts) >= 3 && parts[1] == "manager" {
		return parts[2]
	}
	return ""
}

func convertHomeyTrigger(card *homeyCard) models.Trigger {
	trigger := models.Trigger{
		DeviceID: extractDeviceID(card.URI),
		Config:   make(map[string]any),
	}
	trigger.Config["card_id"] = card.ID
	trigger.Config["uri"] = card.URI

	// Copy relevant args
	for k, v := range card.Args {
		if k != "device" { // Skip device ref, it's in DeviceID
			trigger.Config[k] = v
		}
	}

	mgr := extractManager(card.URI)

	switch {
	case trigger.DeviceID != "":
		// Device-based trigger (e.g., alarm_motion_true, temperature_changed)
		trigger.Type = converter.TriggerDeviceState
	case strings.Contains(card.ID, "sunrise") || strings.Contains(card.ID, "sunset"):
		// Sun events — must check before schedule/cron (sunset is a cron job in Homey)
		trigger.Type = converter.TriggerSun
		trigger.Config["event"] = card.ID
	case mgr == "geofence" || strings.Contains(card.ID, "geofence"):
		trigger.Type = converter.TriggerGeofence
	case mgr == "cron" || strings.Contains(card.ID, "time") || strings.Contains(card.ID, "cron"):
		trigger.Type = converter.TriggerSchedule
	case mgr == "flow" && card.ID == "programmatic_trigger":
		trigger.Type = converter.TriggerWebhook
	default:
		trigger.Type = card.ID // Preserve original card ID
	}

	return trigger
}

func convertHomeyCondition(card *homeyCard) models.Condition {
	cond := models.Condition{
		DeviceID: extractDeviceID(card.URI),
		Config:   make(map[string]any),
	}
	cond.Config["card_id"] = card.ID
	cond.Config["uri"] = card.URI
	if card.Inverted {
		cond.Config["inverted"] = true
	}

	for k, v := range card.Args {
		if k != "device" {
			cond.Config[k] = v
		}
	}

	mgr := extractManager(card.URI)

	switch {
	case cond.DeviceID != "":
		cond.Type = converter.CondDeviceState
	case mgr == "cron" || strings.Contains(card.ID, "time"):
		cond.Type = converter.CondTime
	default:
		cond.Type = card.ID
	}

	return cond
}

func convertHomeyAction(card *homeyCard) models.Action {
	action := models.Action{
		DeviceID: extractDeviceID(card.URI),
		Config:   make(map[string]any),
	}
	action.Config["card_id"] = card.ID
	action.Config["uri"] = card.URI

	for k, v := range card.Args {
		if k != "device" {
			action.Config[k] = v
		}
	}

	mgr := extractManager(card.URI)

	switch {
	case card.ID == "delay" || (mgr == "flow" && card.ID == "delay"):
		action.Type = converter.ActionDelay
		action.DeviceID = "" // Delay is not device-specific
	case strings.Contains(card.ID, "notify") || strings.Contains(card.ID, "notification"):
		action.Type = converter.ActionNotify
	case mgr == "flow" && (card.ID == "run" || card.ID == "run_with_tokens"):
		action.Type = converter.ActionScene // Running another flow ≈ activating a scene
	case action.DeviceID != "":
		action.Type = converter.ActionDeviceCommand
	default:
		action.Type = card.ID
	}

	return action
}

func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}
