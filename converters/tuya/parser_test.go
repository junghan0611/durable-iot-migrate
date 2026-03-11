package tuya

import (
	"testing"

	"github.com/junghan0611/durable-iot-migrate/converters/homeassistant"
	"github.com/junghan0611/durable-iot-migrate/core/converter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tuya scene JSON (from Tuya Cloud API / Cube Private Cloud)
const tuyaJSON = `[
  {
    "scene_id": "tuya-scene-001",
    "name": "Night Mode",
    "enabled": true,
    "conditions": [
      {
        "entity_type": 6,
        "entity_id": "",
        "expr": {"time": "22:00", "timezone_id": "Asia/Seoul"}
      }
    ],
    "preconditions": [
      {
        "cond_type": "timeCheck",
        "expr": {"start": "21:00", "end": "06:00"}
      }
    ],
    "actions": [
      {
        "entity_type": 1,
        "entity_id": "device-light-01",
        "executor_property": {"dp_id": "1", "value": false},
        "action_executor": "dpIssue"
      },
      {
        "entity_type": 1,
        "entity_id": "device-curtain-01",
        "executor_property": {"dp_id": "1", "value": "close"},
        "action_executor": "dpIssue"
      }
    ]
  },
  {
    "scene_id": "tuya-scene-002",
    "name": "Motion Light",
    "enabled": true,
    "conditions": [
      {
        "entity_type": 1,
        "entity_id": "device-pir-01",
        "expr": {"dp_id": "1", "comparator": "==", "value": true}
      }
    ],
    "actions": [
      {
        "entity_type": 1,
        "entity_id": "device-light-02",
        "executor_property": {"dp_id": "1", "value": true},
        "action_executor": "dpIssue"
      },
      {
        "entity_type": 7,
        "entity_id": "",
        "executor_property": {"seconds": 300}
      },
      {
        "entity_type": 1,
        "entity_id": "device-light-02",
        "executor_property": {"dp_id": "1", "value": false},
        "action_executor": "dpIssue"
      }
    ]
  },
  {
    "scene_id": "tuya-scene-003",
    "name": "Sunset Ambiance",
    "enabled": true,
    "conditions": [
      {
        "entity_type": 3,
        "entity_id": "",
        "expr": {"event": "sunset", "offset_minutes": -30}
      }
    ],
    "actions": [
      {
        "entity_type": 3,
        "entity_id": "scene-warm-lights",
        "executor_property": {}
      }
    ]
  },
  {
    "scene_id": "tuya-scene-004",
    "name": "Door Alert",
    "enabled": true,
    "conditions": [
      {
        "entity_type": 1,
        "entity_id": "device-door-sensor-01",
        "expr": {"dp_id": "1", "comparator": "==", "value": true}
      }
    ],
    "actions": [
      {
        "entity_type": 2,
        "entity_id": "",
        "executor_property": {"message": "Front door opened!"}
      }
    ]
  }
]`

func TestParser_ParseBytes(t *testing.T) {
	p := &Parser{}
	assert.Equal(t, "tuya", p.Platform())

	autos, err := p.ParseBytes([]byte(tuyaJSON))
	require.NoError(t, err)
	require.Len(t, autos, 4)

	// Scene 1: Night Mode — timer trigger + time precondition + 2 device actions
	a1 := autos[0]
	assert.Equal(t, "Night Mode", a1.Name)
	assert.True(t, a1.Enabled)
	require.Len(t, a1.Triggers, 1)
	assert.Equal(t, converter.TriggerSchedule, a1.Triggers[0].Type)
	require.Len(t, a1.Conditions, 1)
	assert.Equal(t, converter.CondTime, a1.Conditions[0].Type)
	assert.Equal(t, "21:00", a1.Conditions[0].Config["after"])
	assert.Equal(t, "06:00", a1.Conditions[0].Config["before"])
	require.Len(t, a1.Actions, 2)
	assert.Equal(t, converter.ActionDeviceCommand, a1.Actions[0].Type)
	assert.Equal(t, "device-light-01", a1.Actions[0].DeviceID)
	assert.Equal(t, "device-curtain-01", a1.Actions[1].DeviceID)

	// Scene 2: Motion Light — device trigger + command + delay + command
	a2 := autos[1]
	assert.Equal(t, "Motion Light", a2.Name)
	require.Len(t, a2.Triggers, 1)
	assert.Equal(t, converter.TriggerDeviceState, a2.Triggers[0].Type)
	assert.Equal(t, "device-pir-01", a2.Triggers[0].DeviceID)
	assert.Equal(t, true, a2.Triggers[0].Config["to"])
	require.Len(t, a2.Actions, 3)
	assert.Equal(t, converter.ActionDeviceCommand, a2.Actions[0].Type)
	assert.Equal(t, converter.ActionDelay, a2.Actions[1].Type)
	assert.Equal(t, converter.ActionDeviceCommand, a2.Actions[2].Type)
	// DeviceRefs should have pir and light
	assert.Contains(t, a2.DeviceRefs, "device-pir-01")
	assert.Contains(t, a2.DeviceRefs, "device-light-02")

	// Scene 3: Sunset — sun trigger + scene action
	a3 := autos[2]
	require.Len(t, a3.Triggers, 1)
	assert.Equal(t, converter.TriggerSun, a3.Triggers[0].Type)
	require.Len(t, a3.Actions, 1)
	assert.Equal(t, converter.ActionScene, a3.Actions[0].Type)

	// Scene 4: Door Alert — device trigger + notification
	a4 := autos[3]
	require.Len(t, a4.Actions, 1)
	assert.Equal(t, converter.ActionNotify, a4.Actions[0].Type)
	assert.Equal(t, "Front door opened!", a4.Actions[0].Config["message"])
}

func TestParser_SingleScene(t *testing.T) {
	single := `{
		"scene_id": "tap-001",
		"name": "Tap to Run: All Off",
		"enabled": true,
		"conditions": [],
		"actions": [
			{"entity_type": 1, "entity_id": "dev-01", "executor_property": {"dp_id": "1", "value": false}},
			{"entity_type": 1, "entity_id": "dev-02", "executor_property": {"dp_id": "1", "value": false}}
		]
	}`

	p := &Parser{}
	autos, err := p.ParseBytes([]byte(single))
	require.NoError(t, err)
	require.Len(t, autos, 1)
	assert.Equal(t, "Tap to Run: All Off", autos[0].Name)
	assert.Len(t, autos[0].Actions, 2)
	assert.Empty(t, autos[0].Triggers) // Tap-to-run has no triggers
}

func TestParser_InvalidJSON(t *testing.T) {
	p := &Parser{}
	_, err := p.ParseBytes([]byte("{invalid"))
	require.Error(t, err)
}

func TestCrossConvert_TuyaToHA(t *testing.T) {
	c := &converter.Converter{
		Source: &Parser{},
		Target: &homeassistant.Emitter{},
	}

	out, autos, err := c.Convert([]byte(tuyaJSON))
	require.NoError(t, err)
	assert.Len(t, autos, 4)
	assert.NotEmpty(t, out)

	// Re-parse as HA
	haParser := &homeassistant.Parser{}
	reparsed, err := haParser.ParseBytes(out)
	require.NoError(t, err)
	assert.Len(t, reparsed, 4)

	t.Logf("Tuya JSON → HA YAML:\n%s", string(out))
}

func TestCheckCompatibility_TuyaToSmartThings(t *testing.T) {
	p := &Parser{}
	autos, err := p.ParseBytes([]byte(tuyaJSON))
	require.NoError(t, err)

	report := converter.CheckCompatibility("tuya", "smartthings", autos)
	assert.Equal(t, 4, report.Total)
	assert.Greater(t, report.Converted, 0)

	t.Logf("Tuya→SmartThings: %d converted, %d needs_review, %d dropped",
		report.Converted, report.NeedsReview, report.Dropped)
}

func TestCheckCompatibility_TuyaToGoogle(t *testing.T) {
	p := &Parser{}
	autos, err := p.ParseBytes([]byte(tuyaJSON))
	require.NoError(t, err)

	report := converter.CheckCompatibility("tuya", "google", autos)
	assert.Equal(t, 4, report.Total)

	t.Logf("Tuya→Google: %d converted, %d needs_review, %d dropped",
		report.Converted, report.NeedsReview, report.Dropped)
}
