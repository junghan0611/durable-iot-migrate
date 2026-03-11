package converter

import (
	"strings"

	"github.com/junghan0611/durable-iot-migrate/core/models"
)

// Standard trigger types across platforms.
// Each platform uses different names for the same concept.
//
// Mapping:
//   HA:          trigger.state, trigger.time, trigger.sun, trigger.webhook
//   Tuya:        condition.device (dp change), condition.timer, condition.weather
//   SmartThings: if.equals (device state), if.between (time), every.specific.time
//   Google Home: starter.device.state, starter.time.schedule, starter.sun

const (
	// TriggerDeviceState — a device's attribute changes to a value.
	// HA: trigger: state, entity_id, to/from
	// Tuya: conditions[].entity_type=1 (device DP change)
	// SmartThings: if.equals(device.component.capability.attribute, value)
	// Google Home: starters: device.state
	TriggerDeviceState = "device_state"

	// TriggerSchedule — time-based trigger (cron, specific time, interval).
	// HA: trigger: time, at: "HH:MM"
	// Tuya: conditions[].entity_type=6 (timer)
	// SmartThings: every.specific.time / every.interval
	// Google Home: starters: time.schedule
	TriggerSchedule = "schedule"

	// TriggerSun — sunrise/sunset with optional offset.
	// HA: trigger: sun, event: sunrise/sunset, offset
	// Tuya: conditions[].entity_type=3 (sunrise/sunset)
	// SmartThings: if.between(sunrise, sunset)
	// Google Home: starters: sun
	TriggerSun = "sun"

	// TriggerWebhook — external HTTP trigger.
	// HA: trigger: webhook
	// Tuya: N/A (cloud API only)
	// SmartThings: N/A (requires Connected Service)
	// Google Home: N/A
	TriggerWebhook = "webhook"

	// TriggerGeofence — location-based trigger.
	// HA: trigger: zone, entity_id (device_tracker)
	// Tuya: conditions[].entity_type=5 (geofence)
	// SmartThings: if.equals(presenceSensor.presence, "present")
	// Google Home: starters: home.presence
	TriggerGeofence = "geofence"
)

// Standard condition types.

const (
	// CondDeviceState — check current device attribute value.
	CondDeviceState = "device_state"

	// CondTime — check if current time is within a range.
	CondTime = "time"

	// CondNumeric — check if a numeric value meets a threshold.
	CondNumeric = "numeric"

	// CondZone — check if entity is in a zone.
	CondZone = "zone"

	// CondAnd — all child conditions must be true.
	CondAnd = "and"

	// CondOr — any child condition must be true.
	CondOr = "or"
)

// Standard action types.

const (
	// ActionDeviceCommand — send a command to a device.
	// HA: action: device/service call
	// Tuya: actions[].entity_type=1 (device DP command)
	// SmartThings: command (device.component.capability.command)
	// Google Home: actions: device.command
	ActionDeviceCommand = "device_command"

	// ActionNotify — send a notification.
	// HA: action: notify
	// Tuya: actions[].entity_type=2 (push notification)
	// SmartThings: command (notify)
	// Google Home: actions: assistant.command.Broadcast
	ActionNotify = "notify"

	// ActionDelay — wait before continuing.
	// HA: action: delay
	// Tuya: actions[].entity_type=7 (delay)
	// SmartThings: sleep
	// Google Home: actions: delay
	ActionDelay = "delay"

	// ActionScene — activate another scene/automation.
	// HA: action: scene.turn_on
	// Tuya: actions[].entity_type=3 (scene)
	// SmartThings: command (activateScene)
	// Google Home: actions: device.command.ActivateScene
	ActionScene = "scene"

	// ActionWebhook — call an external URL.
	// HA: action: rest_command / shell_command
	// Tuya: N/A
	// SmartThings: N/A
	// Google Home: N/A
	ActionWebhook = "webhook"
)

// PlatformSupport describes what a platform supports for conversion planning.
type PlatformSupport struct {
	Platform   string
	Triggers   map[string]bool
	Conditions map[string]bool
	Actions    map[string]bool
}

// KnownPlatforms maps platform names to their supported capabilities.
// Used to generate ConversionReport — what gets dropped vs converted.
var KnownPlatforms = map[string]PlatformSupport{
	"homeassistant": {
		Platform: "homeassistant",
		Triggers: map[string]bool{
			TriggerDeviceState: true, TriggerSchedule: true,
			TriggerSun: true, TriggerWebhook: true, TriggerGeofence: true,
		},
		Conditions: map[string]bool{
			CondDeviceState: true, CondTime: true, CondNumeric: true,
			CondZone: true, CondAnd: true, CondOr: true,
		},
		Actions: map[string]bool{
			ActionDeviceCommand: true, ActionNotify: true,
			ActionDelay: true, ActionScene: true, ActionWebhook: true,
		},
	},
	"tuya": {
		Platform: "tuya",
		Triggers: map[string]bool{
			TriggerDeviceState: true, TriggerSchedule: true,
			TriggerSun: true, TriggerGeofence: true,
		},
		Conditions: map[string]bool{
			CondDeviceState: true, CondTime: true, CondNumeric: true,
		},
		Actions: map[string]bool{
			ActionDeviceCommand: true, ActionNotify: true,
			ActionDelay: true, ActionScene: true,
		},
	},
	"smartthings": {
		Platform: "smartthings",
		Triggers: map[string]bool{
			TriggerDeviceState: true, TriggerSchedule: true,
			TriggerSun: true, TriggerGeofence: true,
		},
		Conditions: map[string]bool{
			CondDeviceState: true, CondTime: true, CondNumeric: true,
			CondAnd: true, CondOr: true,
		},
		Actions: map[string]bool{
			ActionDeviceCommand: true, ActionNotify: true,
			ActionDelay: true, ActionScene: true,
		},
	},
	"google": {
		Platform: "google",
		Triggers: map[string]bool{
			TriggerDeviceState: true, TriggerSchedule: true,
			TriggerSun: true, TriggerGeofence: true,
		},
		Conditions: map[string]bool{
			CondDeviceState: true, CondTime: true, CondNumeric: true,
			CondAnd: true,
		},
		Actions: map[string]bool{
			ActionDeviceCommand: true, ActionNotify: true,
			ActionDelay: true, ActionScene: true,
		},
	},
	"homey": {
		Platform: "homey",
		Triggers: map[string]bool{
			TriggerDeviceState: true, TriggerSchedule: true,
			TriggerSun: true, TriggerWebhook: true, TriggerGeofence: true,
		},
		Conditions: map[string]bool{
			CondDeviceState: true, CondTime: true, CondNumeric: true,
			CondAnd: true, CondOr: true,
		},
		Actions: map[string]bool{
			ActionDeviceCommand: true, ActionNotify: true,
			ActionDelay: true, ActionScene: true, ActionWebhook: true,
		},
	},
}

// CheckCompatibility reports what would be lost converting from source to target.
func CheckCompatibility(source, target string, autos []models.Automation) *ConversionReport {
	targetSupport, ok := KnownPlatforms[target]
	if !ok {
		return &ConversionReport{
			SourcePlatform: source,
			TargetPlatform: target,
			Total:          len(autos),
			Dropped:        len(autos),
		}
	}

	report := &ConversionReport{
		SourcePlatform: source,
		TargetPlatform: target,
		Total:          len(autos),
	}

	for _, auto := range autos {
		item := ConversionItem{
			SourceID:   auto.ID,
			SourceName: auto.Name,
			Source:     &auto,
		}

		// Check triggers
		for _, t := range auto.Triggers {
			if !targetSupport.Triggers[t.Type] {
				item.Issues = append(item.Issues,
					"unsupported trigger: "+t.Type)
			}
		}
		// Check conditions
		for _, c := range auto.Conditions {
			if !targetSupport.Conditions[c.Type] {
				item.Issues = append(item.Issues,
					"unsupported condition: "+c.Type)
			}
		}
		// Check actions
		for _, a := range auto.Actions {
			if !targetSupport.Actions[a.Type] {
				item.Issues = append(item.Issues,
					"unsupported action: "+a.Type)
			}
		}

		if len(item.Issues) == 0 {
			item.Status = "converted"
			report.Converted++
		} else {
			// Partial: some unsupported features
			hasBlocker := false
			for _, issue := range item.Issues {
				// Trigger unsupported = can't convert at all
				if strings.HasPrefix(issue, "unsupported trigger:") {
					hasBlocker = true
					break
				}
			}
			if hasBlocker {
				item.Status = "dropped"
				report.Dropped++
			} else {
				item.Status = "needs_review"
				report.NeedsReview++
			}
		}

		report.Items = append(report.Items, item)
	}

	return report
}
