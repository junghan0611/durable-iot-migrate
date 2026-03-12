package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBatchResult_SuccessRate(t *testing.T) {
	tests := []struct {
		name     string
		total    int
		success  int
		expected float64
	}{
		{"all success", 10, 10, 1.0},
		{"half", 10, 5, 0.5},
		{"none", 10, 0, 0.0},
		{"empty batch", 0, 0, 1.0}, // No devices = 100% (nothing to fail)
		{"one of three", 3, 1, 1.0 / 3.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &BatchResult{
				TotalDevices: tt.total,
				Succeeded:    tt.success,
			}
			assert.InDelta(t, tt.expected, r.SuccessRate(), 0.001)
		})
	}
}
