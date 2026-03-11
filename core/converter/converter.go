// Package converter defines the multi-platform automation converter framework.
//
// Every IoT platform represents automations differently, but the underlying
// semantics are universal: trigger → condition → action.
//
//   HA YAML ──→ Parser ──→ core.Automation ──→ Emitter ──→ SmartThings JSON
//   Tuya JSON ──→ Parser ──→ core.Automation ──→ Emitter ──→ Google Home YAML
//
// A Parser reads a platform-native format and produces a core.Automation.
// An Emitter takes a core.Automation and produces a platform-native format.
// Conversion = Parse(source) → Emit(target).
package converter

import "github.com/junghan0611/durable-iot-migrate/core/models"

// Parser reads a platform-native automation format and produces core Automations.
type Parser interface {
	// Platform returns the platform identifier (e.g., "homeassistant", "tuya", "smartthings").
	Platform() string

	// ParseBytes reads raw bytes (YAML, JSON, etc.) and returns parsed automations.
	ParseBytes(data []byte) ([]models.Automation, error)
}

// Emitter takes core Automations and produces a platform-native format.
type Emitter interface {
	// Platform returns the target platform identifier.
	Platform() string

	// EmitBytes converts automations to platform-native format bytes.
	EmitBytes(autos []models.Automation) ([]byte, error)
}

// Converter combines a Parser and Emitter for bidirectional conversion.
type Converter struct {
	Source Parser
	Target Emitter
}

// Convert parses from source format and emits to target format.
func (c *Converter) Convert(sourceData []byte) ([]byte, []models.Automation, error) {
	autos, err := c.Source.ParseBytes(sourceData)
	if err != nil {
		return nil, nil, err
	}
	out, err := c.Target.EmitBytes(autos)
	if err != nil {
		return nil, autos, err
	}
	return out, autos, nil
}

// ConversionReport tracks what was converted, dropped, or needs review.
type ConversionReport struct {
	SourcePlatform string             `json:"source_platform"`
	TargetPlatform string             `json:"target_platform"`
	Total          int                `json:"total"`
	Converted      int                `json:"converted"`
	Dropped        int                `json:"dropped"`
	NeedsReview    int                `json:"needs_review"`
	Items          []ConversionItem   `json:"items"`
}

// ConversionItem tracks the conversion status of a single automation.
type ConversionItem struct {
	SourceID   string      `json:"source_id"`
	SourceName string      `json:"source_name"`
	Status     string      `json:"status"` // "converted", "dropped", "needs_review"
	Issues     []string    `json:"issues,omitempty"`
	Source     *models.Automation `json:"source,omitempty"`
	Result     *models.Automation `json:"result,omitempty"`
}
