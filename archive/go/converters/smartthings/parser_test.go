package smartthings

import (
	"testing"

	"github.com/junghan0611/durable-iot-migrate/converters/homeassistant"
	"github.com/junghan0611/durable-iot-migrate/core/converter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Real SmartThings Rules API JSON from official docs
const stJSON = `[
  {
    "name": "Turn on light when door opens",
    "id": "st-rule-001",
    "actions": [
      {
        "if": {
          "equals": {
            "left": {
              "device": {
                "deviceId": "contact-sensor-01",
                "component": "main",
                "capability": "contactSensor",
                "attribute": "contact"
              }
            },
            "right": {"string": "open"}
          },
          "then": [
            {
              "command": {
                "devices": ["light-01"],
                "commands": [
                  {
                    "component": "main",
                    "capability": "switch",
                    "command": "on"
                  }
                ]
              }
            }
          ],
          "else": [
            {
              "command": {
                "devices": ["light-01"],
                "commands": [
                  {
                    "component": "main",
                    "capability": "switch",
                    "command": "off"
                  }
                ]
              }
            }
          ]
        }
      }
    ]
  },
  {
    "name": "Temperature alert with delay",
    "id": "st-rule-002",
    "actions": [
      {
        "if": {
          "greaterThan": {
            "left": {
              "device": {
                "deviceId": "temp-sensor-01",
                "component": "main",
                "capability": "temperatureMeasurement",
                "attribute": "temperature"
              }
            },
            "right": {"integer": 30}
          },
          "then": [
            {
              "command": {
                "devices": ["ac-01"],
                "commands": [
                  {
                    "component": "main",
                    "capability": "switch",
                    "command": "on"
                  }
                ]
              }
            },
            {
              "sleep": {
                "duration": {"value": 300, "unit": "Second"}
              }
            }
          ]
        }
      }
    ]
  },
  {
    "name": "Sunset routine",
    "id": "st-rule-003",
    "actions": [
      {
        "every": {
          "specific": {
            "reference": "Sunset",
            "offset": {"value": -30, "unit": "Minute"}
          },
          "actions": [
            {
              "command": {
                "devices": ["light-02", "light-03"],
                "commands": [
                  {
                    "component": "main",
                    "capability": "switchLevel",
                    "command": "setLevel",
                    "arguments": [50]
                  }
                ]
              }
            }
          ]
        }
      }
    ]
  }
]`

func TestParser_ParseBytes(t *testing.T) {
	p := &Parser{}
	assert.Equal(t, "smartthings", p.Platform())

	autos, err := p.ParseBytes([]byte(stJSON))
	require.NoError(t, err)
	require.Len(t, autos, 3)

	// Rule 1: Door opens → light on
	a1 := autos[0]
	assert.Equal(t, "Turn on light when door opens", a1.Name)
	require.Len(t, a1.Triggers, 1)
	assert.Equal(t, converter.TriggerDeviceState, a1.Triggers[0].Type)
	assert.Equal(t, "contact-sensor-01", a1.Triggers[0].DeviceID)
	assert.Equal(t, "contactSensor", a1.Triggers[0].Config["capability"])
	assert.Equal(t, "open", a1.Triggers[0].Config["to"])

	// Conditions extracted from if-equals
	require.Len(t, a1.Conditions, 1)
	assert.Equal(t, converter.CondDeviceState, a1.Conditions[0].Type)

	// Then + else both generate actions
	assert.GreaterOrEqual(t, len(a1.Actions), 2)
	assert.Equal(t, converter.ActionDeviceCommand, a1.Actions[0].Type)
	assert.Equal(t, "light-01", a1.Actions[0].DeviceID)
	assert.Equal(t, "on", a1.Actions[0].Config["command"])

	// DeviceRefs
	assert.Contains(t, a1.DeviceRefs, "light-01")

	// Rule 2: Temperature > 30 → AC on + delay
	a2 := autos[1]
	assert.Equal(t, "Temperature alert with delay", a2.Name)
	require.Len(t, a2.Conditions, 1)
	assert.Equal(t, converter.CondNumeric, a2.Conditions[0].Type)
	assert.Equal(t, 30, a2.Conditions[0].Config["above"])

	// Should have command + sleep
	hasDelay := false
	hasCommand := false
	for _, a := range a2.Actions {
		if a.Type == converter.ActionDelay {
			hasDelay = true
		}
		if a.Type == converter.ActionDeviceCommand {
			hasCommand = true
		}
	}
	assert.True(t, hasDelay, "should have delay action")
	assert.True(t, hasCommand, "should have device command")

	// Rule 3: Every sunset
	a3 := autos[2]
	assert.Equal(t, "Sunset routine", a3.Name)
	require.Len(t, a3.Triggers, 1)
	assert.Equal(t, converter.TriggerSun, a3.Triggers[0].Type)
	assert.Equal(t, "Sunset", a3.Triggers[0].Config["event"])

	// Multiple devices in one command
	assert.Contains(t, a3.DeviceRefs, "light-02")
	assert.Contains(t, a3.DeviceRefs, "light-03")
}

func TestParser_SingleRule(t *testing.T) {
	single := `{
		"name": "Simple rule",
		"id": "single-001",
		"actions": [
			{"command": {
				"devices": ["dev-01"],
				"commands": [{"component": "main", "capability": "switch", "command": "on"}]
			}}
		]
	}`

	p := &Parser{}
	autos, err := p.ParseBytes([]byte(single))
	require.NoError(t, err)
	require.Len(t, autos, 1)
	assert.Equal(t, "Simple rule", autos[0].Name)
	require.Len(t, autos[0].Actions, 1)
	assert.Equal(t, "dev-01", autos[0].Actions[0].DeviceID)
}

func TestParser_InvalidJSON(t *testing.T) {
	p := &Parser{}
	_, err := p.ParseBytes([]byte("{invalid"))
	require.Error(t, err)
}

func TestCheckCompatibility_STtoHA(t *testing.T) {
	p := &Parser{}
	autos, err := p.ParseBytes([]byte(stJSON))
	require.NoError(t, err)

	report := converter.CheckCompatibility("smartthings", "homeassistant", autos)
	assert.Equal(t, 3, report.Total)

	// HA supports everything ST supports, so all should convert
	assert.Equal(t, 3, report.Converted)
	assert.Equal(t, 0, report.Dropped)

	t.Logf("ST→HA: %d converted, %d needs_review, %d dropped",
		report.Converted, report.NeedsReview, report.Dropped)
}

func TestCrossConvert_STtoHA(t *testing.T) {
	stParser := &Parser{}
	haEmitter := &homeassistant.Emitter{}

	c := &converter.Converter{
		Source: stParser,
		Target: haEmitter,
	}

	out, autos, err := c.Convert([]byte(stJSON))
	require.NoError(t, err)
	assert.Len(t, autos, 3)
	assert.NotEmpty(t, out)

	// The output should be valid HA YAML
	haParser := &homeassistant.Parser{}
	reparsed, err := haParser.ParseBytes(out)
	require.NoError(t, err)
	assert.Len(t, reparsed, 3)

	t.Logf("SmartThings JSON → HA YAML:\n%s", string(out))
}
