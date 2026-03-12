// Package workflows defines Temporal workflows for IoT migration.
package workflows

import (
	"fmt"
	"time"

	"github.com/junghan0611/durable-iot-migrate/core/activities"
	"github.com/junghan0611/durable-iot-migrate/core/models"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// DeviceMigrationWorkflow orchestrates batch device migration.
// Phase 1: Migrate devices (unbind → bind → verify) with Saga compensation.
// Phase 2: Migrate automations for successfully migrated devices.
func DeviceMigrationWorkflow(ctx workflow.Context, config models.BatchConfig) (models.BatchResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting device migration", "batch_id", config.BatchID)

	if config.Concurrency <= 0 {
		config.Concurrency = 5
	}
	if config.SuccessThreshold <= 0 {
		config.SuccessThreshold = 0.95
	}

	result := models.BatchResult{
		BatchID:   config.BatchID,
		StartedAt: workflow.Now(ctx),
	}

	// Activity options with retry and heartbeat
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		HeartbeatTimeout:    30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var acts *activities.MigrationActivities

	// Phase 1: Fetch devices
	var devices []models.Device
	err := workflow.ExecuteActivity(ctx, acts.FetchDevices).Get(ctx, &devices)
	if err != nil {
		return result, fmt.Errorf("fetch devices failed: %w", err)
	}

	// Filter by config.DeviceIDs if specified
	if len(config.DeviceIDs) > 0 {
		idSet := make(map[string]bool)
		for _, id := range config.DeviceIDs {
			idSet[id] = true
		}
		var filtered []models.Device
		for _, d := range devices {
			if idSet[d.ID] {
				filtered = append(filtered, d)
			}
		}
		devices = filtered
	}

	result.TotalDevices = len(devices)
	logger.Info("Devices to migrate", "count", len(devices))

	// Phase 1: Migrate devices (sequential for now, fan-out later)
	var succeeded []models.Device
	for _, device := range devices {
		var status models.MigrationStatus
		err := workflow.ExecuteActivity(ctx, acts.MigrateDevice, device).Get(ctx, &status)
		if err != nil {
			// Activity-level failure (not migration failure)
			status = models.MigrationStatus{
				DeviceID:   device.ID,
				DeviceName: device.Name,
				Phase:      "failed",
				Error:      fmt.Sprintf("activity error: %v", err),
			}
		}

		result.Devices = append(result.Devices, status)
		if status.Phase == "complete" {
			result.Succeeded++
			succeeded = append(succeeded, device)
		} else {
			result.Failed++
		}
	}

	// Check success threshold
	if result.SuccessRate() < config.SuccessThreshold {
		logger.Warn("Success rate below threshold, halting",
			"rate", result.SuccessRate(),
			"threshold", config.SuccessThreshold,
		)
		// Don't roll back succeeded ones — they're fine.
		// Just stop and report.
		result.CompletedAt = workflow.Now(ctx)
		return result, fmt.Errorf("success rate %.1f%% below threshold %.1f%%",
			result.SuccessRate()*100, config.SuccessThreshold*100)
	}

	// Phase 2: Migrate automations (only if requested and devices succeeded)
	if config.MigrateAutos && len(succeeded) > 0 {
		var autos []models.Automation
		err := workflow.ExecuteActivity(ctx, acts.FetchAutomations).Get(ctx, &autos)
		if err != nil {
			logger.Warn("Fetch automations failed, skipping phase 2", "error", err)
		} else {
			// Filter automations: only those referencing successfully migrated devices
			successIDs := make(map[string]bool)
			for _, d := range succeeded {
				successIDs[d.ID] = true
			}

			for _, auto := range autos {
				canMigrate := true
				for _, ref := range auto.DeviceRefs {
					if !successIDs[ref] {
						canMigrate = false
						break
					}
				}
				if !canMigrate {
					continue
				}

				result.TotalAutos++
				// TODO: Execute automation migration activity
				// For now, count them
				result.AutosSucceeded++
			}
		}
	}

	result.CompletedAt = workflow.Now(ctx)
	logger.Info("Migration batch complete",
		"succeeded", result.Succeeded,
		"failed", result.Failed,
		"success_rate", fmt.Sprintf("%.1f%%", result.SuccessRate()*100),
	)
	return result, nil
}
