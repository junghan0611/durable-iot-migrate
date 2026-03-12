// Package google implements the converter for Google Home scripted automations.
//
// Google Home automation format (YAML-based, from Script Editor):
//
//	metadata:
//	  name: "Turn on lights at sunset"
//	automations:
//	  - starters:
//	      - type: time.schedule
//	        at: sunset
//	        offset: -30min
//	    condition:
//	      type: and
//	      conditions:
//	        - type: device.state
//	          device: Living Room Light
//	          state: on
//	          is: false
//	    actions:
//	      - type: device.command.OnOff
//	        devices: Living Room Light
//	        on: true
package google

import (
	"encoding/json"
	"fmt"

	"github.com/junghan0611/durable-iot-migrate/core/converter"
	"github.com/junghan0611/durable-iot-migrate/core/models"
	"gopkg.in/yaml.v3"
)

// Parser reads Google Home scripted automation YAML.
type Parser struct{}

func (p *Parser) Platform() string { return "google" }

// Top-level structure
type ghScript struct {
	Metadata    ghMetadata     `yaml:"metadata"`
	Automations []ghAutomation `yaml:"automations"`
}

type ghMetadata struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type ghAutomation struct {
	Starters  []ghStarter   `yaml:"starters"`
	Condition *ghCondition  `yaml:"condition"`
	Actions   []ghAction    `yaml:"actions"`
}

type ghStarter struct {
	Type   string         `yaml:"type"`
	At     string         `yaml:"at"`
	Offset string         `yaml:"offset"`
	Device string         `yaml:"device"`
	State  string         `yaml:"state"`
	Is     interface{}    `yaml:"is"`
	Extra  map[string]any `yaml:",inline"`
}

type ghCondition struct {
	Type       string         `yaml:"type"`
	Device     string         `yaml:"device"`
	State      string         `yaml:"state"`
	Is         interface{}    `yaml:"is"`
	Conditions []ghCondition  `yaml:"conditions"`
	Extra      map[string]any `yaml:",inline"`
}

type ghAction struct {
	Type    string         `yaml:"type"`
	Devices interface{}    `yaml:"devices"` // string or []string
	On      *bool          `yaml:"on"`
	Level   *int           `yaml:"level"`
	Extra   map[string]any `yaml:",inline"`
}

func (p *Parser) ParseBytes(data []byte) ([]models.Automation, error) {
	var script ghScript
	if err := yaml.Unmarshal(data, &script); err != nil {
		return nil, fmt.Errorf("parse Google Home YAML: %w", err)
	}

	result := make([]models.Automation, 0, len(script.Automations))
	for i, gh := range script.Automations {
		auto := models.Automation{
			ID:      fmt.Sprintf("gh-auto-%d", i+1),
			Name:    script.Metadata.Name,
			Enabled: true,
		}
		if len(script.Automations) > 1 {
			auto.Name = fmt.Sprintf("%s #%d", script.Metadata.Name, i+1)
		}

		// Starters → Triggers
		for _, s := range gh.Starters {
			trigger := convertGHStarter(s)
			auto.Triggers = append(auto.Triggers, trigger)
			if trigger.DeviceID != "" {
				auto.DeviceRefs = appendUnique(auto.DeviceRefs, trigger.DeviceID)
			}
		}

		// Condition → Conditions
		if gh.Condition != nil {
			conds := convertGHCondition(*gh.Condition)
			auto.Conditions = append(auto.Conditions, conds...)
		}

		// Actions
		for _, a := range gh.Actions {
			action := convertGHAction(a)
			auto.Actions = append(auto.Actions, action)
			if action.DeviceID != "" {
				auto.DeviceRefs = appendUnique(auto.DeviceRefs, action.DeviceID)
			}
		}

		auto.SourceMeta, _ = json.Marshal(gh)
		result = append(result, auto)
	}

	return result, nil
}

func convertGHStarter(s ghStarter) models.Trigger {
	trigger := models.Trigger{Config: make(map[string]any)}

	switch s.Type {
	case "time.schedule":
		if s.At == "sunrise" || s.At == "sunset" {
			trigger.Type = converter.TriggerSun
			trigger.Config["event"] = s.At
		} else {
			trigger.Type = converter.TriggerSchedule
			trigger.Config["at"] = s.At
		}
		if s.Offset != "" {
			trigger.Config["offset"] = s.Offset
		}
	case "device.state":
		trigger.Type = converter.TriggerDeviceState
		trigger.DeviceID = s.Device
		trigger.Config["state"] = s.State
		if s.Is != nil {
			trigger.Config["to"] = s.Is
		}
	case "home.presence":
		trigger.Type = converter.TriggerGeofence
		trigger.Config["state"] = s.State
	default:
		trigger.Type = s.Type
		trigger.Config["raw"] = s.Extra
	}

	return trigger
}

func convertGHCondition(c ghCondition) []models.Condition {
	var result []models.Condition

	switch c.Type {
	case "and":
		for _, child := range c.Conditions {
			result = append(result, convertGHCondition(child)...)
		}
	case "or":
		cond := models.Condition{Type: converter.CondOr, Config: make(map[string]any)}
		result = append(result, cond)
		for _, child := range c.Conditions {
			result = append(result, convertGHCondition(child)...)
		}
	case "device.state":
		cond := models.Condition{
			Type:     converter.CondDeviceState,
			DeviceID: c.Device,
			Config:   map[string]any{"state": c.State},
		}
		if c.Is != nil {
			cond.Config["is"] = c.Is
		}
		result = append(result, cond)
	case "time":
		cond := models.Condition{
			Type:   converter.CondTime,
			Config: map[string]any{},
		}
		for k, v := range c.Extra {
			cond.Config[k] = v
		}
		result = append(result, cond)
	default:
		cond := models.Condition{
			Type:   c.Type,
			Config: map[string]any{"raw": c.Extra},
		}
		result = append(result, cond)
	}

	return result
}

func convertGHAction(a ghAction) models.Action {
	action := models.Action{Config: make(map[string]any)}
	action.DeviceID = deviceString(a.Devices)

	switch {
	case len(a.Type) > 15 && a.Type[:15] == "device.command.":
		action.Type = converter.ActionDeviceCommand
		action.Config["command_type"] = a.Type
		if a.On != nil {
			action.Config["on"] = *a.On
		}
		if a.Level != nil {
			action.Config["level"] = *a.Level
		}
	case a.Type == "assistant.command.Broadcast":
		action.Type = converter.ActionNotify
		action.Config["command_type"] = a.Type
	case a.Type == "device.command.ActivateScene":
		action.Type = converter.ActionScene
		action.Config["command_type"] = a.Type
	case a.Type == "delay":
		action.Type = converter.ActionDelay
	default:
		action.Type = a.Type
	}

	for k, v := range a.Extra {
		action.Config[k] = v
	}

	return action
}

func deviceString(v interface{}) string {
	switch d := v.(type) {
	case string:
		return d
	case []interface{}:
		if len(d) > 0 {
			if s, ok := d[0].(string); ok {
				return s
			}
		}
	}
	return ""
}

func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}
