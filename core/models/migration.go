package models

import "time"

// BatchConfig defines a migration batch.
type BatchConfig struct {
	BatchID          string   `json:"batch_id"`
	SourcePlatform   string   `json:"source_platform"`   // e.g., "homeassistant", "thingsboard"
	TargetPlatform   string   `json:"target_platform"`
	DeviceIDs        []string `json:"device_ids"`        // Specific devices to migrate (empty = all)
	MigrateAutos     bool     `json:"migrate_autos"`     // Also migrate automations?
	Concurrency      int      `json:"concurrency"`       // Parallel device migrations (default: 5)
	SuccessThreshold float64  `json:"success_threshold"` // Min success rate to continue (0.95 = 95%)
}

// BatchResult summarizes a completed batch.
type BatchResult struct {
	BatchID         string             `json:"batch_id"`
	TotalDevices    int                `json:"total_devices"`
	Succeeded       int                `json:"succeeded"`
	Failed          int                `json:"failed"`
	RolledBack      int                `json:"rolled_back"`
	TotalAutos      int                `json:"total_autos"`
	AutosSucceeded  int                `json:"autos_succeeded"`
	AutosFailed     int                `json:"autos_failed"`
	Devices         []MigrationStatus  `json:"devices"`
	StartedAt       time.Time          `json:"started_at"`
	CompletedAt     time.Time          `json:"completed_at"`
	TemporalRunID   string             `json:"temporal_run_id,omitempty"`
}

// SuccessRate returns the device migration success rate.
func (r *BatchResult) SuccessRate() float64 {
	if r.TotalDevices == 0 {
		return 1.0
	}
	return float64(r.Succeeded) / float64(r.TotalDevices)
}
