// Package homeassistant implements the converter for Home Assistant automation YAML.
//
// HA automation format:
//
//	- id: "1234"
//	  alias: "Turn on lights at sunset"
//	  triggers:
//	    - trigger: sun
//	      event: sunset
//	      offset: "-01:00:00"
//	  conditions:
//	    - condition: state
//	      entity_id: group.people
//	      state: "home"
//	  actions:
//	    - action: light.turn_on
//	      target:
//	        entity_id: light.living_room
package homeassistant

import (
	"encoding/json"
	"fmt"

	"github.com/junghan0611/durable-iot-migrate/core/converter"
	"github.com/junghan0611/durable-iot-migrate/core/models"
	"gopkg.in/yaml.v3"
)

// Parser reads Home Assistant automation YAML.
type Parser struct{}

func (p *Parser) Platform() string { return "homeassistant" }

// haAutomation represents a single HA automation in YAML.
type haAutomation struct {
	ID          string        `yaml:"id"`
	Alias       string        `yaml:"alias"`
	Description string        `yaml:"description"`
	Mode        string        `yaml:"mode"`
	Triggers    []haTrigger   `yaml:"triggers"`
	Conditions  []haCondition `yaml:"conditions"`
	Actions     []haAction    `yaml:"actions"`
}

type haTrigger struct {
	Trigger  string         `yaml:"trigger"`
	Platform string         `yaml:"platform"` // Legacy field (pre-2024.10)
	EntityID interface{}    `yaml:"entity_id"`
	Event    string         `yaml:"event"`
	Offset   string         `yaml:"offset"`
	To       interface{}    `yaml:"to"`
	From     interface{}    `yaml:"from"`
	At       string         `yaml:"at"`
	Extra    map[string]any `yaml:",inline"`
}

type haCondition struct {
	Condition string         `yaml:"condition"`
	EntityID  interface{}    `yaml:"entity_id"`
	State     interface{}    `yaml:"state"`
	After     string         `yaml:"after"`
	Before    string         `yaml:"before"`
	Below     interface{}    `yaml:"below"`
	Above     interface{}    `yaml:"above"`
	Extra     map[string]any `yaml:",inline"`
}

type haAction struct {
	Action   string         `yaml:"action"`
	Service  string         `yaml:"service"` // Legacy field
	Target   *haTarget      `yaml:"target"`
	Data     map[string]any `yaml:"data"`
	Delay    interface{}    `yaml:"delay"`
	Scene    string         `yaml:"scene"`
	Extra    map[string]any `yaml:",inline"`
}

type haTarget struct {
	EntityID interface{} `yaml:"entity_id"`
	DeviceID interface{} `yaml:"device_id"`
	AreaID   interface{} `yaml:"area_id"`
}

func (p *Parser) ParseBytes(data []byte) ([]models.Automation, error) {
	var haAutos []haAutomation
	if err := yaml.Unmarshal(data, &haAutos); err != nil {
		return nil, fmt.Errorf("parse HA YAML: %w", err)
	}

	result := make([]models.Automation, 0, len(haAutos))
	for _, ha := range haAutos {
		auto := models.Automation{
			ID:      ha.ID,
			Name:    ha.Alias,
			Enabled: true,
		}

		// Parse triggers
		for _, t := range ha.Triggers {
			trigger := convertHATrigger(t)
			auto.Triggers = append(auto.Triggers, trigger)
			if trigger.DeviceID != "" {
				auto.DeviceRefs = appendUnique(auto.DeviceRefs, trigger.DeviceID)
			}
		}

		// Parse conditions
		for _, c := range ha.Conditions {
			cond := convertHACondition(c)
			auto.Conditions = append(auto.Conditions, cond)
		}

		// Parse actions
		for _, a := range ha.Actions {
			action := convertHAAction(a)
			auto.Actions = append(auto.Actions, action)
			if action.DeviceID != "" {
				auto.DeviceRefs = appendUnique(auto.DeviceRefs, action.DeviceID)
			}
		}

		// Store original YAML as source_meta
		raw, _ := json.Marshal(ha)
		auto.SourceMeta = raw

		result = append(result, auto)
	}
	return result, nil
}

func convertHATrigger(t haTrigger) models.Trigger {
	// Use "trigger" field, fall back to "platform" (legacy)
	trigType := t.Trigger
	if trigType == "" {
		trigType = t.Platform
	}

	trigger := models.Trigger{
		Config: make(map[string]any),
	}

	switch trigType {
	case "state":
		trigger.Type = converter.TriggerDeviceState
		trigger.DeviceID = entityToString(t.EntityID)
		if t.To != nil {
			trigger.Config["to"] = t.To
		}
		if t.From != nil {
			trigger.Config["from"] = t.From
		}
	case "time":
		trigger.Type = converter.TriggerSchedule
		trigger.Config["at"] = t.At
	case "sun":
		trigger.Type = converter.TriggerSun
		trigger.Config["event"] = t.Event
		if t.Offset != "" {
			trigger.Config["offset"] = t.Offset
		}
	case "webhook":
		trigger.Type = converter.TriggerWebhook
	case "zone":
		trigger.Type = converter.TriggerGeofence
		trigger.DeviceID = entityToString(t.EntityID)
		trigger.Config["event"] = t.Event
	default:
		// Unknown trigger type — preserve as-is
		trigger.Type = trigType
		trigger.Config["raw"] = t.Extra
	}

	return trigger
}

func convertHACondition(c haCondition) models.Condition {
	cond := models.Condition{
		Config: make(map[string]any),
	}

	switch c.Condition {
	case "state":
		cond.Type = converter.CondDeviceState
		cond.DeviceID = entityToString(c.EntityID)
		if c.State != nil {
			cond.Config["state"] = c.State
		}
	case "time":
		cond.Type = converter.CondTime
		if c.After != "" {
			cond.Config["after"] = c.After
		}
		if c.Before != "" {
			cond.Config["before"] = c.Before
		}
	case "numeric_state":
		cond.Type = converter.CondNumeric
		cond.DeviceID = entityToString(c.EntityID)
		if c.Above != nil {
			cond.Config["above"] = c.Above
		}
		if c.Below != nil {
			cond.Config["below"] = c.Below
		}
	case "zone":
		cond.Type = converter.CondZone
		cond.DeviceID = entityToString(c.EntityID)
	case "and":
		cond.Type = converter.CondAnd
	case "or":
		cond.Type = converter.CondOr
	default:
		cond.Type = c.Condition
		cond.Config["raw"] = c.Extra
	}

	return cond
}

func convertHAAction(a haAction) models.Action {
	action := models.Action{
		Config: make(map[string]any),
	}

	// Handle delay
	if a.Delay != nil {
		action.Type = converter.ActionDelay
		action.Config["delay"] = a.Delay
		return action
	}

	// Handle scene
	if a.Scene != "" {
		action.Type = converter.ActionScene
		action.Config["scene"] = a.Scene
		return action
	}

	// Use "action" field, fall back to "service" (legacy)
	svc := a.Action
	if svc == "" {
		svc = a.Service
	}

	switch {
	case svc == "notify" || (len(svc) > 7 && svc[:7] == "notify."):
		action.Type = converter.ActionNotify
		action.Config["service"] = svc
		if a.Data != nil {
			action.Config["data"] = a.Data
		}
	case svc != "":
		action.Type = converter.ActionDeviceCommand
		action.Config["service"] = svc
		if a.Target != nil {
			action.DeviceID = targetToString(a.Target)
			action.Config["target"] = a.Target
		}
		if a.Data != nil {
			action.Config["data"] = a.Data
		}
	default:
		action.Type = "unknown"
		action.Config["raw"] = a.Extra
	}

	return action
}

func entityToString(v interface{}) string {
	switch e := v.(type) {
	case string:
		return e
	case []interface{}:
		if len(e) > 0 {
			if s, ok := e[0].(string); ok {
				return s
			}
		}
	}
	return ""
}

func targetToString(t *haTarget) string {
	if s := entityToString(t.EntityID); s != "" {
		return s
	}
	if s := entityToString(t.DeviceID); s != "" {
		return s
	}
	return entityToString(t.AreaID)
}

func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}
