package converter

import (
	"fmt"
	"testing"

	"github.com/junghan0611/durable-iot-migrate/core/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockParser for testing
type mockParser struct{}

func (p *mockParser) Platform() string { return "mock" }
func (p *mockParser) ParseBytes(data []byte) ([]models.Automation, error) {
	if string(data) == "error" {
		return nil, fmt.Errorf("parse error")
	}
	return []models.Automation{
		{ID: "1", Name: "Test Auto", Triggers: []models.Trigger{{Type: TriggerDeviceState}},
			Actions: []models.Action{{Type: ActionDeviceCommand}}},
	}, nil
}

type mockEmitter struct{ fail bool }

func (e *mockEmitter) Platform() string { return "mock" }
func (e *mockEmitter) EmitBytes(autos []models.Automation) ([]byte, error) {
	if e.fail {
		return nil, fmt.Errorf("emit error")
	}
	return []byte("output"), nil
}

func TestConverter_Convert(t *testing.T) {
	c := &Converter{Source: &mockParser{}, Target: &mockEmitter{}}
	out, autos, err := c.Convert([]byte("input"))
	require.NoError(t, err)
	assert.Equal(t, []byte("output"), out)
	assert.Len(t, autos, 1)
}

func TestConverter_ParseError(t *testing.T) {
	c := &Converter{Source: &mockParser{}, Target: &mockEmitter{}}
	_, _, err := c.Convert([]byte("error"))
	require.Error(t, err)
}

func TestConverter_EmitError(t *testing.T) {
	c := &Converter{Source: &mockParser{}, Target: &mockEmitter{fail: true}}
	_, autos, err := c.Convert([]byte("input"))
	require.Error(t, err)
	assert.Len(t, autos, 1) // Parsed successfully, emit failed
}

func TestCheckCompatibility_AllSupported(t *testing.T) {
	autos := []models.Automation{
		{
			ID: "1", Name: "Simple",
			Triggers:   []models.Trigger{{Type: TriggerDeviceState}},
			Conditions: []models.Condition{{Type: CondDeviceState}},
			Actions:    []models.Action{{Type: ActionDeviceCommand}},
		},
	}
	report := CheckCompatibility("any", "homeassistant", autos)
	assert.Equal(t, 1, report.Converted)
	assert.Equal(t, 0, report.Dropped)
	assert.Equal(t, 0, report.NeedsReview)
}

func TestCheckCompatibility_UnsupportedTrigger(t *testing.T) {
	autos := []models.Automation{
		{
			ID: "1", Name: "Webhook Auto",
			Triggers: []models.Trigger{{Type: TriggerWebhook}},
			Actions:  []models.Action{{Type: ActionDeviceCommand}},
		},
	}
	// Tuya doesn't support webhook triggers
	report := CheckCompatibility("ha", "tuya", autos)
	assert.Equal(t, 1, report.Dropped)
	assert.Equal(t, 0, report.Converted)
}

func TestCheckCompatibility_UnsupportedAction(t *testing.T) {
	autos := []models.Automation{
		{
			ID: "1", Name: "Webhook Action",
			Triggers: []models.Trigger{{Type: TriggerDeviceState}},
			Actions:  []models.Action{{Type: ActionWebhook}},
		},
	}
	// Tuya doesn't support webhook actions → needs_review (not dropped, trigger is OK)
	report := CheckCompatibility("ha", "tuya", autos)
	assert.Equal(t, 0, report.Dropped)
	assert.Equal(t, 1, report.NeedsReview)
}

func TestCheckCompatibility_UnknownTarget(t *testing.T) {
	autos := []models.Automation{{ID: "1"}, {ID: "2"}}
	report := CheckCompatibility("ha", "unknown", autos)
	assert.Equal(t, 2, report.Dropped)
}

func TestCheckCompatibility_Empty(t *testing.T) {
	report := CheckCompatibility("ha", "tuya", nil)
	assert.Equal(t, 0, report.Total)
	assert.Equal(t, 0, report.Converted)
}

func TestKnownPlatforms_Coverage(t *testing.T) {
	expected := []string{"homeassistant", "tuya", "smartthings", "google"}
	for _, p := range expected {
		support, ok := KnownPlatforms[p]
		assert.True(t, ok, "platform %s should be known", p)
		assert.Equal(t, p, support.Platform)
		assert.NotEmpty(t, support.Triggers)
		assert.NotEmpty(t, support.Conditions)
		assert.NotEmpty(t, support.Actions)
	}
}

// Test the full matrix: all platforms can convert to all other platforms
func TestConversionMatrix(t *testing.T) {
	autos := []models.Automation{
		{
			ID: "matrix-1", Name: "Simple Light",
			Triggers:   []models.Trigger{{Type: TriggerDeviceState, DeviceID: "dev-1"}},
			Conditions: []models.Condition{{Type: CondTime}},
			Actions:    []models.Action{{Type: ActionDeviceCommand, DeviceID: "dev-1"}},
		},
		{
			ID: "matrix-2", Name: "Sunset Scene",
			Triggers: []models.Trigger{{Type: TriggerSun}},
			Actions:  []models.Action{{Type: ActionScene}},
		},
	}

	platforms := []string{"homeassistant", "tuya", "smartthings", "google"}
	for _, src := range platforms {
		for _, tgt := range platforms {
			if src == tgt {
				continue
			}
			report := CheckCompatibility(src, tgt, autos)
			t.Logf("%s→%s: %d/%d converted, %d review, %d dropped",
				src, tgt, report.Converted, report.Total, report.NeedsReview, report.Dropped)

			// These two automations use universally supported types
			assert.Equal(t, 2, report.Converted, "%s→%s", src, tgt)
		}
	}
}
