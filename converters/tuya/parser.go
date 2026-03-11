// Package tuya implements the converter for Tuya scene/tap-to-run JSON.
//
// Tuya scene format (from Cloud API):
//
//	{
//	  "scene_id": "abc123",
//	  "name": "Night Mode",
//	  "conditions": [
//	    {"entity_type": 1, "entity_id": "device-01", "expr": {"dp_id": "1", "comparator": "==", "value": true}}
//	  ],
//	  "preconditions": [
//	    {"cond_type": "timeCheck", "expr": {"start": "22:00", "end": "06:00"}}
//	  ],
//	  "actions": [
//	    {"entity_type": 1, "entity_id": "device-02", "executor_property": {"dp_id": "1", "value": false}}
//	  ]
//	}
//
// entity_type: 1=device, 2=notification, 3=weather, 5=geofence, 6=timer, 7=delay
package tuya

import (
	"encoding/json"
	"fmt"

	"github.com/junghan0611/durable-iot-migrate/core/converter"
	"github.com/junghan0611/durable-iot-migrate/core/models"
)

// Parser reads Tuya scene JSON.
type Parser struct{}

func (p *Parser) Platform() string { return "tuya" }

type tuyaScene struct {
	SceneID       string             `json:"scene_id"`
	Name          string             `json:"name"`
	Enabled       bool               `json:"enabled"`
	Conditions    []tuyaCondition    `json:"conditions"`
	Preconditions []tuyaPrecondition `json:"preconditions"`
	Actions       []tuyaAction       `json:"actions"`
}

type tuyaCondition struct {
	EntityType int            `json:"entity_type"`
	EntityID   string         `json:"entity_id"`
	Expr       map[string]any `json:"expr"`
}

type tuyaPrecondition struct {
	CondType string         `json:"cond_type"`
	Expr     map[string]any `json:"expr"`
}

type tuyaAction struct {
	EntityType       int            `json:"entity_type"`
	EntityID         string         `json:"entity_id"`
	ExecutorProperty map[string]any `json:"executor_property"`
	ActionExecutor   string         `json:"action_executor"`
}

func (p *Parser) ParseBytes(data []byte) ([]models.Automation, error) {
	var scenes []tuyaScene
	if err := json.Unmarshal(data, &scenes); err != nil {
		var single tuyaScene
		if err2 := json.Unmarshal(data, &single); err2 != nil {
			return nil, fmt.Errorf("parse Tuya JSON: %w", err)
		}
		scenes = []tuyaScene{single}
	}

	result := make([]models.Automation, 0, len(scenes))
	for _, scene := range scenes {
		auto := models.Automation{
			ID:      scene.SceneID,
			Name:    scene.Name,
			Enabled: scene.Enabled,
		}

		// Conditions (triggers in Tuya terminology)
		for _, c := range scene.Conditions {
			trigger := convertTuyaCondition(c)
			auto.Triggers = append(auto.Triggers, trigger)
			if trigger.DeviceID != "" {
				auto.DeviceRefs = appendUnique(auto.DeviceRefs, trigger.DeviceID)
			}
		}

		// Preconditions (guard conditions)
		for _, pc := range scene.Preconditions {
			cond := convertTuyaPrecondition(pc)
			auto.Conditions = append(auto.Conditions, cond)
		}

		// Actions
		for _, a := range scene.Actions {
			action := convertTuyaAction(a)
			auto.Actions = append(auto.Actions, action)
			if action.DeviceID != "" {
				auto.DeviceRefs = appendUnique(auto.DeviceRefs, action.DeviceID)
			}
		}

		auto.SourceMeta, _ = json.Marshal(scene)
		result = append(result, auto)
	}

	return result, nil
}

// Tuya entity_type mapping:
// 1 = device DP, 2 = notification, 3 = weather/sunrise/sunset,
// 5 = geofence, 6 = timer/schedule, 7 = delay

func convertTuyaCondition(c tuyaCondition) models.Trigger {
	trigger := models.Trigger{
		DeviceID: c.EntityID,
		Config:   make(map[string]any),
	}

	switch c.EntityType {
	case 1: // Device DP change
		trigger.Type = converter.TriggerDeviceState
		if c.Expr != nil {
			trigger.Config["dp_id"] = c.Expr["dp_id"]
			trigger.Config["comparator"] = c.Expr["comparator"]
			trigger.Config["to"] = c.Expr["value"]
		}
	case 3: // Weather / sunrise / sunset
		trigger.Type = converter.TriggerSun
		trigger.DeviceID = ""
		for k, v := range c.Expr {
			trigger.Config[k] = v
		}
	case 5: // Geofence
		trigger.Type = converter.TriggerGeofence
		for k, v := range c.Expr {
			trigger.Config[k] = v
		}
	case 6: // Timer
		trigger.Type = converter.TriggerSchedule
		for k, v := range c.Expr {
			trigger.Config[k] = v
		}
	default:
		trigger.Type = fmt.Sprintf("tuya_entity_%d", c.EntityType)
		trigger.Config["raw"] = c.Expr
	}

	return trigger
}

func convertTuyaPrecondition(pc tuyaPrecondition) models.Condition {
	cond := models.Condition{
		Config: make(map[string]any),
	}

	switch pc.CondType {
	case "timeCheck":
		cond.Type = converter.CondTime
		if start, ok := pc.Expr["start"]; ok {
			cond.Config["after"] = start
		}
		if end, ok := pc.Expr["end"]; ok {
			cond.Config["before"] = end
		}
	case "deviceCheck":
		cond.Type = converter.CondDeviceState
		for k, v := range pc.Expr {
			cond.Config[k] = v
		}
	default:
		cond.Type = pc.CondType
		cond.Config["raw"] = pc.Expr
	}

	return cond
}

func convertTuyaAction(a tuyaAction) models.Action {
	action := models.Action{
		DeviceID: a.EntityID,
		Config:   make(map[string]any),
	}

	switch a.EntityType {
	case 1: // Device command
		action.Type = converter.ActionDeviceCommand
		if a.ExecutorProperty != nil {
			action.Config["dp_id"] = a.ExecutorProperty["dp_id"]
			action.Config["value"] = a.ExecutorProperty["value"]
		}
		if a.ActionExecutor != "" {
			action.Config["executor"] = a.ActionExecutor
		}
	case 2: // Notification
		action.Type = converter.ActionNotify
		action.DeviceID = ""
		if a.ExecutorProperty != nil {
			for k, v := range a.ExecutorProperty {
				action.Config[k] = v
			}
		}
	case 3: // Activate scene
		action.Type = converter.ActionScene
		action.Config["scene_id"] = a.EntityID
	case 7: // Delay
		action.Type = converter.ActionDelay
		action.DeviceID = ""
		if a.ExecutorProperty != nil {
			action.Config["seconds"] = a.ExecutorProperty["seconds"]
			action.Config["minutes"] = a.ExecutorProperty["minutes"]
		}
	default:
		action.Type = fmt.Sprintf("tuya_action_%d", a.EntityType)
		action.Config["raw"] = a.ExecutorProperty
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
