package homey

import (
	"testing"

	"github.com/junghan0611/durable-iot-migrate/converters/homeassistant"
	"github.com/junghan0611/durable-iot-migrate/core/converter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Homey Flow JSON (from Homey Pro Web API / backup)
const homeyJSON = `[
  {
    "id": "flow-001",
    "name": "Motion Light",
    "enabled": true,
    "trigger": {
      "uri": "homey:device:pir-sensor-01",
      "id": "alarm_motion_true",
      "args": {"device": {"id": "pir-sensor-01", "name": "PIR Sensor"}}
    },
    "conditions": [
      {
        "uri": "homey:device:light-01",
        "id": "onoff",
        "args": {"device": {"id": "light-01", "name": "Living Room Light"}},
        "inverted": true
      }
    ],
    "actions": [
      {
        "uri": "homey:device:light-01",
        "id": "on",
        "args": {"device": {"id": "light-01", "name": "Living Room Light"}}
      },
      {
        "uri": "homey:manager:flow",
        "id": "delay",
        "args": {"delay": 300}
      },
      {
        "uri": "homey:device:light-01",
        "id": "off",
        "args": {"device": {"id": "light-01", "name": "Living Room Light"}}
      }
    ]
  },
  {
    "id": "flow-002",
    "name": "Sunset Lights",
    "enabled": true,
    "trigger": {
      "uri": "homey:manager:cron",
      "id": "sunset",
      "args": {"offset": -30}
    },
    "conditions": [],
    "actions": [
      {
        "uri": "homey:device:light-02",
        "id": "on",
        "args": {"device": {"id": "light-02"}, "brightness": 0.7}
      },
      {
        "uri": "homey:device:light-03",
        "id": "on",
        "args": {"device": {"id": "light-03"}, "brightness": 0.5}
      }
    ]
  },
  {
    "id": "flow-003",
    "name": "Good Morning Routine",
    "enabled": true,
    "trigger": {
      "uri": "homey:manager:cron",
      "id": "time",
      "args": {"time": "07:00"}
    },
    "conditions": [
      {
        "uri": "homey:manager:cron",
        "id": "time_between",
        "args": {"startTime": "06:00", "endTime": "09:00"}
      }
    ],
    "actions": [
      {
        "uri": "homey:device:thermostat-01",
        "id": "set_target_temperature",
        "args": {"device": {"id": "thermostat-01"}, "temperature": 22}
      },
      {
        "uri": "homey:manager:notifications",
        "id": "notify",
        "args": {"text": "Good morning! Temperature set to 22°C"}
      }
    ]
  },
  {
    "id": "flow-004",
    "name": "Run Scene Flow",
    "enabled": true,
    "trigger": {
      "uri": "homey:device:button-01",
      "id": "button_pressed",
      "args": {"device": {"id": "button-01"}}
    },
    "conditions": [],
    "actions": [
      {
        "uri": "homey:manager:flow",
        "id": "run",
        "args": {"flow": {"id": "flow-night-scene", "name": "Night Scene"}}
      }
    ]
  },
  {
    "id": "flow-005",
    "name": "Geofence Away",
    "enabled": false,
    "trigger": {
      "uri": "homey:manager:geofence",
      "id": "user_left",
      "args": {"user": "user-01"}
    },
    "conditions": [],
    "actions": [
      {
        "uri": "homey:device:lock-01",
        "id": "locked",
        "args": {"device": {"id": "lock-01"}}
      }
    ]
  }
]`

func TestParser_ParseBytes(t *testing.T) {
	p := &Parser{}
	assert.Equal(t, "homey", p.Platform())

	autos, err := p.ParseBytes([]byte(homeyJSON))
	require.NoError(t, err)
	require.Len(t, autos, 5)

	// Flow 1: Motion Light — device trigger + inverted condition + command + delay + command
	a1 := autos[0]
	assert.Equal(t, "Motion Light", a1.Name)
	assert.True(t, a1.Enabled)
	require.Len(t, a1.Triggers, 1)
	assert.Equal(t, converter.TriggerDeviceState, a1.Triggers[0].Type)
	assert.Equal(t, "pir-sensor-01", a1.Triggers[0].DeviceID)
	assert.Equal(t, "alarm_motion_true", a1.Triggers[0].Config["card_id"])

	require.Len(t, a1.Conditions, 1)
	assert.Equal(t, converter.CondDeviceState, a1.Conditions[0].Type)
	assert.Equal(t, true, a1.Conditions[0].Config["inverted"])

	require.Len(t, a1.Actions, 3)
	assert.Equal(t, converter.ActionDeviceCommand, a1.Actions[0].Type)
	assert.Equal(t, "light-01", a1.Actions[0].DeviceID)
	assert.Equal(t, converter.ActionDelay, a1.Actions[1].Type)
	assert.Equal(t, "", a1.Actions[1].DeviceID) // Delay has no device
	assert.Equal(t, converter.ActionDeviceCommand, a1.Actions[2].Type)

	assert.Contains(t, a1.DeviceRefs, "pir-sensor-01")
	assert.Contains(t, a1.DeviceRefs, "light-01")

	// Flow 2: Sunset — sun trigger
	a2 := autos[1]
	assert.Equal(t, "Sunset Lights", a2.Name)
	require.Len(t, a2.Triggers, 1)
	assert.Equal(t, converter.TriggerSun, a2.Triggers[0].Type)
	assert.Len(t, a2.Actions, 2)

	// Flow 3: Schedule + time condition + notify
	a3 := autos[2]
	assert.Equal(t, converter.TriggerSchedule, a3.Triggers[0].Type)
	require.Len(t, a3.Conditions, 1)
	assert.Equal(t, converter.CondTime, a3.Conditions[0].Type)
	assert.Len(t, a3.Actions, 2)
	assert.Equal(t, converter.ActionDeviceCommand, a3.Actions[0].Type)
	assert.Equal(t, converter.ActionNotify, a3.Actions[1].Type)

	// Flow 4: Button press → run another flow (scene)
	a4 := autos[3]
	assert.Equal(t, converter.TriggerDeviceState, a4.Triggers[0].Type)
	require.Len(t, a4.Actions, 1)
	assert.Equal(t, converter.ActionScene, a4.Actions[0].Type)

	// Flow 5: Geofence (disabled)
	a5 := autos[4]
	assert.False(t, a5.Enabled)
	assert.Equal(t, converter.TriggerGeofence, a5.Triggers[0].Type)
	assert.Equal(t, converter.ActionDeviceCommand, a5.Actions[0].Type)
	assert.Equal(t, "lock-01", a5.Actions[0].DeviceID)
}

func TestParser_SingleFlow(t *testing.T) {
	single := `{
		"id": "single-001",
		"name": "Simple Toggle",
		"enabled": true,
		"trigger": {
			"uri": "homey:device:switch-01",
			"id": "onoff_true",
			"args": {"device": {"id": "switch-01"}}
		},
		"conditions": [],
		"actions": [
			{"uri": "homey:device:light-01", "id": "toggle", "args": {"device": {"id": "light-01"}}}
		]
	}`

	p := &Parser{}
	autos, err := p.ParseBytes([]byte(single))
	require.NoError(t, err)
	require.Len(t, autos, 1)
	assert.Equal(t, "Simple Toggle", autos[0].Name)
}

func TestParser_NoTrigger(t *testing.T) {
	// Tap-to-run equivalent in Homey (manual flow)
	manual := `{
		"id": "manual-001",
		"name": "All Lights Off",
		"enabled": true,
		"trigger": null,
		"conditions": [],
		"actions": [
			{"uri": "homey:device:light-01", "id": "off", "args": {}},
			{"uri": "homey:device:light-02", "id": "off", "args": {}}
		]
	}`

	p := &Parser{}
	autos, err := p.ParseBytes([]byte(manual))
	require.NoError(t, err)
	require.Len(t, autos, 1)
	assert.Empty(t, autos[0].Triggers)
	assert.Len(t, autos[0].Actions, 2)
}

func TestParser_InvalidJSON(t *testing.T) {
	p := &Parser{}
	_, err := p.ParseBytes([]byte("{invalid"))
	require.Error(t, err)
}

func TestCrossConvert_HomeyToHA(t *testing.T) {
	c := &converter.Converter{
		Source: &Parser{},
		Target: &homeassistant.Emitter{},
	}

	out, autos, err := c.Convert([]byte(homeyJSON))
	require.NoError(t, err)
	assert.Len(t, autos, 5)
	assert.NotEmpty(t, out)

	// Re-parse as HA
	haParser := &homeassistant.Parser{}
	reparsed, err := haParser.ParseBytes(out)
	require.NoError(t, err)
	assert.Len(t, reparsed, 5)

	t.Logf("Homey → HA YAML:\n%s", string(out))
}

func TestCheckCompatibility_HomeyToHA(t *testing.T) {
	p := &Parser{}
	autos, err := p.ParseBytes([]byte(homeyJSON))
	require.NoError(t, err)

	report := converter.CheckCompatibility("homey", "homeassistant", autos)
	assert.Equal(t, 5, report.Total)
	// All standard types should be supported by HA
	assert.Equal(t, 5, report.Converted)

	t.Logf("Homey→HA: %d converted, %d needs_review, %d dropped",
		report.Converted, report.NeedsReview, report.Dropped)
}

func TestCheckCompatibility_HomeyToTuya(t *testing.T) {
	p := &Parser{}
	autos, err := p.ParseBytes([]byte(homeyJSON))
	require.NoError(t, err)

	report := converter.CheckCompatibility("homey", "tuya", autos)
	assert.Equal(t, 5, report.Total)
	// Webhook trigger would be dropped, but our test data doesn't have one
	assert.Greater(t, report.Converted, 0)

	t.Logf("Homey→Tuya: %d converted, %d needs_review, %d dropped",
		report.Converted, report.NeedsReview, report.Dropped)
}
