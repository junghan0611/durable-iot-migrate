package google

import (
	"testing"

	"github.com/junghan0611/durable-iot-migrate/converters/homeassistant"
	"github.com/junghan0611/durable-iot-migrate/core/converter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Google Home scripted automation YAML (from Script Editor)
const ghYAML = `
metadata:
  name: "Evening routine"
  description: "Turn on lights and set temperature at sunset"
automations:
  - starters:
      - type: time.schedule
        at: sunset
        offset: "-30min"
    condition:
      type: and
      conditions:
        - type: device.state
          device: Living Room Light
          state: on
          is: false
    actions:
      - type: device.command.OnOff
        devices: Living Room Light
        on: true
      - type: device.command.BrightnessAbsolute
        devices: Living Room Light
        level: 70

  - starters:
      - type: device.state
        device: Front Door Sensor
        state: contact
        is: open
    actions:
      - type: device.command.OnOff
        devices: Hallway Light
        on: true
`

func TestParser_ParseBytes(t *testing.T) {
	p := &Parser{}
	assert.Equal(t, "google", p.Platform())

	autos, err := p.ParseBytes([]byte(ghYAML))
	require.NoError(t, err)
	require.Len(t, autos, 2)

	// Automation 1: Sunset trigger + condition + 2 actions
	a1 := autos[0]
	assert.Equal(t, "Evening routine #1", a1.Name) // Named with index since >1
	require.Len(t, a1.Triggers, 1)
	assert.Equal(t, converter.TriggerSun, a1.Triggers[0].Type)
	assert.Equal(t, "sunset", a1.Triggers[0].Config["event"])
	assert.Equal(t, "-30min", a1.Triggers[0].Config["offset"])

	// Condition: device.state (and is flattened)
	require.Len(t, a1.Conditions, 1)
	assert.Equal(t, converter.CondDeviceState, a1.Conditions[0].Type)
	assert.Equal(t, "Living Room Light", a1.Conditions[0].DeviceID)

	// Actions: OnOff + Brightness
	require.Len(t, a1.Actions, 2)
	assert.Equal(t, converter.ActionDeviceCommand, a1.Actions[0].Type)
	assert.Equal(t, true, a1.Actions[0].Config["on"])
	assert.Equal(t, converter.ActionDeviceCommand, a1.Actions[1].Type)
	assert.Equal(t, 70, a1.Actions[1].Config["level"])

	// DeviceRefs
	assert.Contains(t, a1.DeviceRefs, "Living Room Light")

	// Automation 2: Device state trigger
	a2 := autos[1]
	assert.Equal(t, "Evening routine #2", a2.Name)
	require.Len(t, a2.Triggers, 1)
	assert.Equal(t, converter.TriggerDeviceState, a2.Triggers[0].Type)
	assert.Equal(t, "Front Door Sensor", a2.Triggers[0].DeviceID)
	assert.Equal(t, "open", a2.Triggers[0].Config["to"])
}

func TestParser_SingleAutomation(t *testing.T) {
	single := `
metadata:
  name: "Simple light"
automations:
  - starters:
      - type: time.schedule
        at: "22:00"
    actions:
      - type: device.command.OnOff
        devices: Bedroom Light
        on: false
`
	p := &Parser{}
	autos, err := p.ParseBytes([]byte(single))
	require.NoError(t, err)
	require.Len(t, autos, 1)
	assert.Equal(t, "Simple light", autos[0].Name) // No index suffix for single
	assert.Equal(t, converter.TriggerSchedule, autos[0].Triggers[0].Type)
}

func TestParser_PresenceStarter(t *testing.T) {
	presence := `
metadata:
  name: "Welcome home"
automations:
  - starters:
      - type: home.presence
        state: someone_home
    actions:
      - type: device.command.OnOff
        devices: Entryway Light
        on: true
`
	p := &Parser{}
	autos, err := p.ParseBytes([]byte(presence))
	require.NoError(t, err)
	assert.Equal(t, converter.TriggerGeofence, autos[0].Triggers[0].Type)
}

func TestParser_InvalidYAML(t *testing.T) {
	p := &Parser{}
	_, err := p.ParseBytes([]byte("{invalid: yaml: ["))
	require.Error(t, err)
}

func TestCrossConvert_GoogleToHA(t *testing.T) {
	c := &converter.Converter{
		Source: &Parser{},
		Target: &homeassistant.Emitter{},
	}

	out, autos, err := c.Convert([]byte(ghYAML))
	require.NoError(t, err)
	assert.Len(t, autos, 2)
	assert.NotEmpty(t, out)

	// Re-parse as HA
	haParser := &homeassistant.Parser{}
	reparsed, err := haParser.ParseBytes(out)
	require.NoError(t, err)
	assert.Len(t, reparsed, 2)

	t.Logf("Google Home → HA YAML:\n%s", string(out))
}

func TestCheckCompatibility_GoogleToHA(t *testing.T) {
	p := &Parser{}
	autos, err := p.ParseBytes([]byte(ghYAML))
	require.NoError(t, err)

	report := converter.CheckCompatibility("google", "homeassistant", autos)
	assert.Equal(t, 2, report.Total)
	assert.Equal(t, 2, report.Converted)
	assert.Equal(t, 0, report.Dropped)
}
