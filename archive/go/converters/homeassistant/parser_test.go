package homeassistant

import (
	"testing"

	"github.com/junghan0611/durable-iot-migrate/core/converter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Real HA automation YAML from official docs
const haYAML = `
- id: "1001"
  alias: "Turn on lights at sunset"
  triggers:
    - trigger: sun
      event: sunset
      offset: "-01:00:00"
  conditions:
    - condition: state
      entity_id: group.people
      state: "home"
  actions:
    - action: light.turn_on
      target:
        entity_id: light.living_room

- id: "1002"
  alias: "Morning routine"
  triggers:
    - trigger: time
      at: "07:00:00"
  conditions:
    - condition: time
      after: "06:00:00"
      before: "08:00:00"
  actions:
    - action: light.turn_on
      target:
        entity_id: light.bedroom
      data:
        brightness_pct: 50
    - delay: "00:05:00"
    - action: media_player.play_media
      target:
        entity_id: media_player.kitchen
      data:
        media_content_id: "http://radio.example.com"
        media_content_type: "music"

- id: "1003"
  alias: "Doorbell camera notification"
  triggers:
    - trigger: state
      entity_id: binary_sensor.doorbell
      to: "on"
  actions:
    - action: notify.mobile_app
      data:
        message: "Someone is at the door!"
        data:
          image: "/api/camera_proxy/camera.front_door"

- id: "1004"
  alias: "Temperature alert"
  triggers:
    - trigger: state
      entity_id: sensor.temperature
  conditions:
    - condition: numeric_state
      entity_id: sensor.temperature
      above: 30
  actions:
    - action: climate.set_temperature
      target:
        entity_id: climate.living_room
      data:
        temperature: 24
    - action: notify.notify
      data:
        message: "Temperature above 30°C, AC activated"

- id: "1005"
  alias: "Activate scene at night"
  triggers:
    - trigger: time
      at: "22:00:00"
  actions:
    - scene: scene.good_night
`

func TestParser_ParseBytes(t *testing.T) {
	p := &Parser{}
	assert.Equal(t, "homeassistant", p.Platform())

	autos, err := p.ParseBytes([]byte(haYAML))
	require.NoError(t, err)
	require.Len(t, autos, 5)

	// Automation 1: Sun trigger
	a1 := autos[0]
	assert.Equal(t, "1001", a1.ID)
	assert.Equal(t, "Turn on lights at sunset", a1.Name)
	require.Len(t, a1.Triggers, 1)
	assert.Equal(t, converter.TriggerSun, a1.Triggers[0].Type)
	assert.Equal(t, "sunset", a1.Triggers[0].Config["event"])
	assert.Equal(t, "-01:00:00", a1.Triggers[0].Config["offset"])

	require.Len(t, a1.Conditions, 1)
	assert.Equal(t, converter.CondDeviceState, a1.Conditions[0].Type)
	assert.Equal(t, "group.people", a1.Conditions[0].DeviceID)

	require.Len(t, a1.Actions, 1)
	assert.Equal(t, converter.ActionDeviceCommand, a1.Actions[0].Type)
	assert.Equal(t, "light.living_room", a1.Actions[0].DeviceID)
	assert.Contains(t, a1.DeviceRefs, "light.living_room")

	// Automation 2: Schedule + delay
	a2 := autos[1]
	assert.Equal(t, "Morning routine", a2.Name)
	require.Len(t, a2.Triggers, 1)
	assert.Equal(t, converter.TriggerSchedule, a2.Triggers[0].Type)
	require.Len(t, a2.Actions, 3)
	assert.Equal(t, converter.ActionDeviceCommand, a2.Actions[0].Type)
	assert.Equal(t, converter.ActionDelay, a2.Actions[1].Type)
	assert.Equal(t, converter.ActionDeviceCommand, a2.Actions[2].Type)

	// Automation 3: Device state trigger + notify
	a3 := autos[2]
	assert.Equal(t, "Doorbell camera notification", a3.Name)
	assert.Equal(t, converter.TriggerDeviceState, a3.Triggers[0].Type)
	assert.Equal(t, "binary_sensor.doorbell", a3.Triggers[0].DeviceID)
	assert.Equal(t, converter.ActionNotify, a3.Actions[0].Type)

	// Automation 4: Numeric condition
	a4 := autos[3]
	require.Len(t, a4.Conditions, 1)
	assert.Equal(t, converter.CondNumeric, a4.Conditions[0].Type)
	assert.Equal(t, 30, a4.Conditions[0].Config["above"])
	assert.Len(t, a4.Actions, 2)
	assert.Equal(t, converter.ActionDeviceCommand, a4.Actions[0].Type)
	assert.Equal(t, converter.ActionNotify, a4.Actions[1].Type)

	// Automation 5: Scene
	a5 := autos[4]
	require.Len(t, a5.Actions, 1)
	assert.Equal(t, converter.ActionScene, a5.Actions[0].Type)
	assert.Equal(t, "scene.good_night", a5.Actions[0].Config["scene"])
}

func TestParser_LegacyFormat(t *testing.T) {
	// HA pre-2024.10 used "platform" instead of "trigger", "service" instead of "action"
	legacy := `
- id: "legacy-001"
  alias: "Legacy automation"
  triggers:
    - platform: state
      entity_id: switch.garage
      to: "off"
  actions:
    - service: switch.turn_on
      target:
        entity_id: switch.garage
`
	p := &Parser{}
	autos, err := p.ParseBytes([]byte(legacy))
	require.NoError(t, err)
	require.Len(t, autos, 1)

	assert.Equal(t, converter.TriggerDeviceState, autos[0].Triggers[0].Type)
	assert.Equal(t, converter.ActionDeviceCommand, autos[0].Actions[0].Type)
}

func TestParser_EmptyInput(t *testing.T) {
	p := &Parser{}
	autos, err := p.ParseBytes([]byte("[]"))
	require.NoError(t, err)
	assert.Empty(t, autos)
}

func TestParser_InvalidYAML(t *testing.T) {
	p := &Parser{}
	_, err := p.ParseBytes([]byte("{invalid: [yaml"))
	require.Error(t, err)
}

func TestEmitter_RoundTrip(t *testing.T) {
	parser := &Parser{}
	emitter := &Emitter{}

	// Parse original
	autos, err := parser.ParseBytes([]byte(haYAML))
	require.NoError(t, err)

	// Emit back to YAML
	out, err := emitter.EmitBytes(autos)
	require.NoError(t, err)

	// Re-parse emitted YAML
	autos2, err := parser.ParseBytes(out)
	require.NoError(t, err)

	// Same count
	assert.Len(t, autos2, len(autos))

	// Same structure (trigger/condition/action types)
	for i := range autos {
		assert.Equal(t, autos[i].Name, autos2[i].Name, "automation %d name", i)
		assert.Len(t, autos2[i].Triggers, len(autos[i].Triggers), "automation %d triggers", i)
		assert.Len(t, autos2[i].Conditions, len(autos[i].Conditions), "automation %d conditions", i)
		assert.Len(t, autos2[i].Actions, len(autos[i].Actions), "automation %d actions", i)

		for j := range autos[i].Triggers {
			assert.Equal(t, autos[i].Triggers[j].Type, autos2[i].Triggers[j].Type)
		}
		for j := range autos[i].Actions {
			assert.Equal(t, autos[i].Actions[j].Type, autos2[i].Actions[j].Type)
		}
	}
}

func TestConverter_HAtoHA(t *testing.T) {
	c := &converter.Converter{
		Source: &Parser{},
		Target: &Emitter{},
	}

	out, autos, err := c.Convert([]byte(haYAML))
	require.NoError(t, err)
	assert.Len(t, autos, 5)
	assert.NotEmpty(t, out)
}

func TestCheckCompatibility_HAtoTuya(t *testing.T) {
	parser := &Parser{}
	autos, err := parser.ParseBytes([]byte(haYAML))
	require.NoError(t, err)

	report := converter.CheckCompatibility("homeassistant", "tuya", autos)
	assert.Equal(t, 5, report.Total)
	assert.Greater(t, report.Converted, 0)

	// Automation with webhook trigger would be dropped (Tuya doesn't support webhook)
	// Our test data doesn't have webhook, so all should convert or need_review
	t.Logf("HA→Tuya: %d converted, %d needs_review, %d dropped",
		report.Converted, report.NeedsReview, report.Dropped)

	// Webhook action in REST command would need review
	for _, item := range report.Items {
		if item.Status != "converted" {
			t.Logf("  %s (%s): %v", item.SourceName, item.Status, item.Issues)
		}
	}
}

func TestCheckCompatibility_HAtoUnknown(t *testing.T) {
	parser := &Parser{}
	autos, err := parser.ParseBytes([]byte(haYAML))
	require.NoError(t, err)

	report := converter.CheckCompatibility("homeassistant", "unknown_platform", autos)
	assert.Equal(t, 5, report.Total)
	assert.Equal(t, 5, report.Dropped)
	assert.Equal(t, 0, report.Converted)
}
