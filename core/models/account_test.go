package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccountBatchResult_AccountSuccessRate(t *testing.T) {
	tests := []struct {
		name     string
		total    int
		success  int
		expected float64
	}{
		{"all success", 100, 100, 1.0},
		{"half", 100, 50, 0.5},
		{"empty", 0, 0, 1.0},
		{"one failure", 1000, 999, 0.999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := AccountBatchResult{
				TotalAccounts:   tt.total,
				AccountsSuccess: tt.success,
			}
			assert.InDelta(t, tt.expected, r.AccountSuccessRate(), 0.001)
		})
	}
}

func TestAccountBatchResult_HasCriticalFailures(t *testing.T) {
	r := AccountBatchResult{CriticalFailed: 0}
	assert.False(t, r.HasCriticalFailures())

	r.CriticalFailed = 1
	assert.True(t, r.HasCriticalFailures())
}

func TestCategorySafetyClass(t *testing.T) {
	tests := []struct {
		category string
		expected SafetyClass
	}{
		{"camera", SafetyCritical},
		{"lock", SafetyCritical},
		{"smoke_detector", SafetyCritical},
		{"co_detector", SafetyCritical},
		{"alarm", SafetyCritical},
		{"doorbell", SafetyCritical},
		{"thermostat", SafetyImportant},
		{"garage", SafetyImportant},
		{"water_valve", SafetyImportant},
		{"gas_valve", SafetyImportant},
		{"light", SafetyNormal},
		{"sensor", SafetyNormal},
		{"plug", SafetyNormal},
		{"fan", SafetyNormal},
		{"unknown", SafetyNormal},
	}

	for _, tt := range tests {
		t.Run(tt.category, func(t *testing.T) {
			assert.Equal(t, tt.expected, CategorySafetyClass(tt.category))
		})
	}
}

func TestDevice_IsCritical(t *testing.T) {
	d := Device{SafetyClass: SafetyCritical}
	assert.True(t, d.IsCritical())

	d.SafetyClass = SafetyNormal
	assert.False(t, d.IsCritical())

	d.SafetyClass = SafetyImportant
	assert.False(t, d.IsCritical())
}
