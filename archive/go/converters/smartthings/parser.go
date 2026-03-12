// Package smartthings implements the converter for SmartThings Rules API JSON.
//
// SmartThings Rules format:
//
//	{
//	  "name": "Turn on light when door opens",
//	  "actions": [{
//	    "if": {
//	      "equals": {
//	        "left": {"device": {"deviceId": "...", "component": "main",
//	                 "capability": "contactSensor", "attribute": "contact"}},
//	        "right": {"string": "open"}
//	      },
//	      "then": [{"command": {"devices": ["..."],
//	                "commands": [{"component": "main", "capability": "switch",
//	                              "command": "on"}]}}]
//	    }
//	  }]
//	}
package smartthings

import (
	"encoding/json"
	"fmt"

	"github.com/junghan0611/durable-iot-migrate/core/converter"
	"github.com/junghan0611/durable-iot-migrate/core/models"
)

// Parser reads SmartThings Rules API JSON.
type Parser struct{}

func (p *Parser) Platform() string { return "smartthings" }

// Top-level rule
type stRule struct {
	Name    string     `json:"name"`
	ID      string     `json:"id"`
	Actions []stAction `json:"actions"`
}

// An action can be: if, command, sleep, every
type stAction struct {
	If      *stIf      `json:"if,omitempty"`
	Command *stCommand `json:"command,omitempty"`
	Sleep   *stSleep   `json:"sleep,omitempty"`
	Every   *stEvery   `json:"every,omitempty"`
}

type stIf struct {
	Equals             *stEquals  `json:"equals,omitempty"`
	GreaterThan        *stCompare `json:"greaterThan,omitempty"`
	LessThan           *stCompare `json:"lessThan,omitempty"`
	GreaterThanOrEquals *stCompare `json:"greaterThanOrEquals,omitempty"`
	LessThanOrEquals   *stCompare `json:"lessThanOrEquals,omitempty"`
	Then               []stAction `json:"then,omitempty"`
	Else               []stAction `json:"else,omitempty"`
}

type stEquals struct {
	Left  stOperand `json:"left"`
	Right stOperand `json:"right"`
}

type stCompare struct {
	Left  stOperand `json:"left"`
	Right stOperand `json:"right"`
}

type stOperand struct {
	Device  *stDeviceRef `json:"device,omitempty"`
	String  *string      `json:"string,omitempty"`
	Integer *int         `json:"integer,omitempty"`
	Double  *float64     `json:"double,omitempty"`
	Boolean *bool        `json:"boolean,omitempty"`
}

type stDeviceRef struct {
	DeviceID   string `json:"deviceId"`
	Component  string `json:"component"`
	Capability string `json:"capability"`
	Attribute  string `json:"attribute"`
}

type stCommand struct {
	Devices  []string      `json:"devices"`
	Commands []stDeviceCmd `json:"commands"`
}

type stDeviceCmd struct {
	Component  string         `json:"component"`
	Capability string         `json:"capability"`
	Command    string         `json:"command"`
	Arguments  []interface{}  `json:"arguments,omitempty"`
}

type stSleep struct {
	Duration stDuration `json:"duration"`
}

type stDuration struct {
	Value json.Number `json:"value"`
	Unit  string      `json:"unit"` // "Second", "Minute"
}

type stEvery struct {
	Specific *stSpecific `json:"specific,omitempty"`
	Interval *stInterval `json:"interval,omitempty"`
	Actions  []stAction  `json:"actions,omitempty"`
}

type stSpecific struct {
	Reference string   `json:"reference"` // "Now", "Sunrise", "Sunset"
	Offset    *stOffset `json:"offset,omitempty"`
}

type stOffset struct {
	Value json.Number `json:"value"`
	Unit  string      `json:"unit"`
}

type stInterval struct {
	Value json.Number `json:"value"`
	Unit  string      `json:"unit"`
}

func (p *Parser) ParseBytes(data []byte) ([]models.Automation, error) {
	// Try single rule first, then array
	var rules []stRule
	if err := json.Unmarshal(data, &rules); err != nil {
		var single stRule
		if err2 := json.Unmarshal(data, &single); err2 != nil {
			return nil, fmt.Errorf("parse SmartThings JSON: %w", err)
		}
		rules = []stRule{single}
	}

	result := make([]models.Automation, 0, len(rules))
	for _, rule := range rules {
		auto := models.Automation{
			ID:      rule.ID,
			Name:    rule.Name,
			Enabled: true,
		}

		parseSTActions(rule.Actions, &auto)
		auto.SourceMeta, _ = json.Marshal(rule)
		result = append(result, auto)
	}
	return result, nil
}

func parseSTActions(actions []stAction, auto *models.Automation) {
	for _, action := range actions {
		if action.If != nil {
			parseSTIf(action.If, auto)
		}
		if action.Command != nil {
			parseSTCommand(action.Command, auto)
		}
		if action.Sleep != nil {
			auto.Actions = append(auto.Actions, models.Action{
				Type:   converter.ActionDelay,
				Config: map[string]any{"duration": action.Sleep.Duration.Value.String(), "unit": action.Sleep.Duration.Unit},
			})
		}
		if action.Every != nil {
			parseSTEvery(action.Every, auto)
		}
	}
}

func parseSTIf(ifAction *stIf, auto *models.Automation) {
	// Extract trigger from condition (equals, greaterThan, etc.)
	if ifAction.Equals != nil {
		t, c := parseSTEquals(ifAction.Equals)
		if t.Type != "" {
			auto.Triggers = append(auto.Triggers, t)
		}
		if c.Type != "" {
			auto.Conditions = append(auto.Conditions, c)
		}
	}
	if ifAction.GreaterThan != nil {
		auto.Conditions = append(auto.Conditions, parseSTCompare(ifAction.GreaterThan, "above"))
	}
	if ifAction.LessThan != nil {
		auto.Conditions = append(auto.Conditions, parseSTCompare(ifAction.LessThan, "below"))
	}

	// Then actions
	parseSTActions(ifAction.Then, auto)
	// Else actions
	parseSTActions(ifAction.Else, auto)
}

func parseSTEquals(eq *stEquals) (models.Trigger, models.Condition) {
	trigger := models.Trigger{Config: make(map[string]any)}
	cond := models.Condition{Config: make(map[string]any)}

	if eq.Left.Device != nil {
		dev := eq.Left.Device
		trigger.Type = converter.TriggerDeviceState
		trigger.DeviceID = dev.DeviceID
		trigger.Config["capability"] = dev.Capability
		trigger.Config["attribute"] = dev.Attribute
		trigger.Config["component"] = dev.Component

		rightVal := operandValue(eq.Right)
		trigger.Config["to"] = rightVal

		// Also create condition (SmartThings if-equals is both trigger and condition)
		cond.Type = converter.CondDeviceState
		cond.DeviceID = dev.DeviceID
		cond.Config["capability"] = dev.Capability
		cond.Config["attribute"] = dev.Attribute
		cond.Config["state"] = rightVal
	}

	return trigger, cond
}

func parseSTCompare(cmp *stCompare, direction string) models.Condition {
	cond := models.Condition{
		Type:   converter.CondNumeric,
		Config: make(map[string]any),
	}

	if cmp.Left.Device != nil {
		cond.DeviceID = cmp.Left.Device.DeviceID
		cond.Config["capability"] = cmp.Left.Device.Capability
		cond.Config["attribute"] = cmp.Left.Device.Attribute
	}

	cond.Config[direction] = operandValue(cmp.Right)
	return cond
}

func parseSTCommand(cmd *stCommand, auto *models.Automation) {
	for _, dev := range cmd.Devices {
		for _, c := range cmd.Commands {
			action := models.Action{
				Type:     converter.ActionDeviceCommand,
				DeviceID: dev,
				Config: map[string]any{
					"capability": c.Capability,
					"command":    c.Command,
					"component":  c.Component,
				},
			}
			if len(c.Arguments) > 0 {
				action.Config["arguments"] = c.Arguments
			}
			auto.Actions = append(auto.Actions, action)
			auto.DeviceRefs = appendUnique(auto.DeviceRefs, dev)
		}
	}
}

func parseSTEvery(every *stEvery, auto *models.Automation) {
	if every.Specific != nil {
		trigger := models.Trigger{
			Config: make(map[string]any),
		}
		switch every.Specific.Reference {
		case "Sunrise", "Sunset":
			trigger.Type = converter.TriggerSun
			trigger.Config["event"] = every.Specific.Reference
		default:
			trigger.Type = converter.TriggerSchedule
			trigger.Config["reference"] = every.Specific.Reference
		}
		if every.Specific.Offset != nil {
			trigger.Config["offset_value"] = every.Specific.Offset.Value.String()
			trigger.Config["offset_unit"] = every.Specific.Offset.Unit
		}
		auto.Triggers = append(auto.Triggers, trigger)
	}
	if every.Interval != nil {
		auto.Triggers = append(auto.Triggers, models.Trigger{
			Type:   converter.TriggerSchedule,
			Config: map[string]any{"interval": every.Interval.Value.String(), "unit": every.Interval.Unit},
		})
	}

	parseSTActions(every.Actions, auto)
}

func operandValue(op stOperand) interface{} {
	if op.String != nil {
		return *op.String
	}
	if op.Integer != nil {
		return *op.Integer
	}
	if op.Double != nil {
		return *op.Double
	}
	if op.Boolean != nil {
		return *op.Boolean
	}
	return nil
}

func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}
