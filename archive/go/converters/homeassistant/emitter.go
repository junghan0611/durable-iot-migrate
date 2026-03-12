package homeassistant

import (
	"github.com/junghan0611/durable-iot-migrate/core/converter"
	"github.com/junghan0611/durable-iot-migrate/core/models"
	"gopkg.in/yaml.v3"
)

// Emitter writes Home Assistant automation YAML.
type Emitter struct{}

func (e *Emitter) Platform() string { return "homeassistant" }

func (e *Emitter) EmitBytes(autos []models.Automation) ([]byte, error) {
	haAutos := make([]haAutomation, 0, len(autos))

	for _, auto := range autos {
		ha := haAutomation{
			ID:    auto.ID,
			Alias: auto.Name,
			Mode:  "single",
		}

		for _, t := range auto.Triggers {
			ha.Triggers = append(ha.Triggers, emitTrigger(t))
		}
		for _, c := range auto.Conditions {
			ha.Conditions = append(ha.Conditions, emitCondition(c))
		}
		for _, a := range auto.Actions {
			ha.Actions = append(ha.Actions, emitAction(a))
		}

		haAutos = append(haAutos, ha)
	}

	return yaml.Marshal(haAutos)
}

func emitTrigger(t models.Trigger) haTrigger {
	ht := haTrigger{}

	switch t.Type {
	case converter.TriggerDeviceState:
		ht.Trigger = "state"
		ht.EntityID = t.DeviceID
		if v, ok := t.Config["to"]; ok {
			ht.To = v
		}
		if v, ok := t.Config["from"]; ok {
			ht.From = v
		}
	case converter.TriggerSchedule:
		ht.Trigger = "time"
		if v, ok := t.Config["at"].(string); ok {
			ht.At = v
		}
	case converter.TriggerSun:
		ht.Trigger = "sun"
		if v, ok := t.Config["event"].(string); ok {
			ht.Event = v
		}
		if v, ok := t.Config["offset"].(string); ok {
			ht.Offset = v
		}
	case converter.TriggerWebhook:
		ht.Trigger = "webhook"
	case converter.TriggerGeofence:
		ht.Trigger = "zone"
		ht.EntityID = t.DeviceID
	default:
		ht.Trigger = t.Type
	}

	return ht
}

func emitCondition(c models.Condition) haCondition {
	hc := haCondition{}

	switch c.Type {
	case converter.CondDeviceState:
		hc.Condition = "state"
		hc.EntityID = c.DeviceID
		if v, ok := c.Config["state"]; ok {
			hc.State = v
		}
	case converter.CondTime:
		hc.Condition = "time"
		if v, ok := c.Config["after"].(string); ok {
			hc.After = v
		}
		if v, ok := c.Config["before"].(string); ok {
			hc.Before = v
		}
	case converter.CondNumeric:
		hc.Condition = "numeric_state"
		hc.EntityID = c.DeviceID
		if v, ok := c.Config["above"]; ok {
			hc.Above = v
		}
		if v, ok := c.Config["below"]; ok {
			hc.Below = v
		}
	case converter.CondZone:
		hc.Condition = "zone"
		hc.EntityID = c.DeviceID
	case converter.CondAnd:
		hc.Condition = "and"
	case converter.CondOr:
		hc.Condition = "or"
	default:
		hc.Condition = c.Type
	}

	return hc
}

func emitAction(a models.Action) haAction {
	ha := haAction{}

	switch a.Type {
	case converter.ActionDeviceCommand:
		if svc, ok := a.Config["service"].(string); ok {
			ha.Action = svc
		} else {
			ha.Action = "homeassistant.turn_on"
		}
		if a.DeviceID != "" {
			ha.Target = &haTarget{EntityID: a.DeviceID}
		}
		if data, ok := a.Config["data"].(map[string]any); ok {
			ha.Data = data
		}
	case converter.ActionNotify:
		if svc, ok := a.Config["service"].(string); ok {
			ha.Action = svc
		} else {
			ha.Action = "notify.notify"
		}
		if data, ok := a.Config["data"].(map[string]any); ok {
			ha.Data = data
		}
	case converter.ActionDelay:
		ha.Delay = a.Config["delay"]
	case converter.ActionScene:
		if scene, ok := a.Config["scene"].(string); ok {
			ha.Scene = scene
		}
	case converter.ActionWebhook:
		ha.Action = "rest_command.call"
		if data, ok := a.Config["data"].(map[string]any); ok {
			ha.Data = data
		}
	default:
		ha.Action = a.Type
	}

	return ha
}
