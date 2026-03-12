package activities

import (
	"context"
	"fmt"

	"github.com/junghan0611/durable-iot-migrate/core/models"
	"go.temporal.io/sdk/activity"
)

// MigrationActivities holds the platform adapters and exposes Temporal activities.
type MigrationActivities struct {
	Source SourcePlatform
	Target TargetPlatform
}

// FetchDevices retrieves the device list from the source platform.
func (a *MigrationActivities) FetchDevices(ctx context.Context) ([]models.Device, error) {
	activity.GetLogger(ctx).Info("Fetching devices from source platform")
	return a.Source.ListDevices(ctx)
}

// FetchAutomations retrieves the automation list from the source platform.
func (a *MigrationActivities) FetchAutomations(ctx context.Context) ([]models.Automation, error) {
	activity.GetLogger(ctx).Info("Fetching automations from source platform")
	return a.Source.ListAutomations(ctx)
}

// MigrateDevice performs a single device migration: unbind → bind → verify.
// Returns the migration status. Does NOT return error on migration failure —
// instead, returns a status with Phase="failed" so the workflow can decide
// whether to continue or roll back.
func (a *MigrationActivities) MigrateDevice(ctx context.Context, device models.Device) (models.MigrationStatus, error) {
	logger := activity.GetLogger(ctx)
	status := models.MigrationStatus{
		DeviceID:   device.ID,
		DeviceName: device.Name,
		Attempts:   1,
	}

	// Step 1: Unbind from source
	logger.Info("Unbinding device from source", "device_id", device.ID)
	status.Phase = "unbind"
	activity.RecordHeartbeat(ctx, status)

	if err := a.Source.UnbindDevice(ctx, device.ID); err != nil {
		status.Phase = "failed"
		status.Error = fmt.Sprintf("unbind failed: %v", err)
		return status, nil // Return status, not error — let workflow decide
	}

	// Step 2: Bind to target
	logger.Info("Binding device to target", "device_id", device.ID)
	status.Phase = "bind"
	activity.RecordHeartbeat(ctx, status)

	if err := a.Target.BindDevice(ctx, device); err != nil {
		// Compensation: rebind to source
		logger.Warn("Bind failed, compensating: rebinding to source", "device_id", device.ID)
		_ = a.Source.RebindDevice(ctx, device.ID) // Best effort
		status.Phase = "failed"
		status.Error = fmt.Sprintf("bind failed: %v", err)
		return status, nil
	}

	// Step 3: Verify on target
	logger.Info("Verifying device on target", "device_id", device.ID)
	status.Phase = "verify"
	activity.RecordHeartbeat(ctx, status)

	ok, err := a.Target.VerifyDevice(ctx, device.ID)
	if err != nil || !ok {
		// Compensation: unbind from target, rebind to source
		logger.Warn("Verification failed, compensating", "device_id", device.ID)
		_ = a.Target.UnbindDevice(ctx, device.ID)
		_ = a.Source.RebindDevice(ctx, device.ID)
		status.Phase = "failed"
		if err != nil {
			status.Error = fmt.Sprintf("verify failed: %v", err)
		} else {
			status.Error = "verify failed: device not reachable"
		}
		return status, nil
	}

	status.Phase = "complete"
	logger.Info("Device migration complete", "device_id", device.ID)
	return status, nil
}

// RollbackDevice compensates a completed migration: unbind from target, rebind to source.
func (a *MigrationActivities) RollbackDevice(ctx context.Context, deviceID string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Rolling back device", "device_id", deviceID)

	if err := a.Target.UnbindDevice(ctx, deviceID); err != nil {
		logger.Warn("Target unbind failed during rollback", "device_id", deviceID, "error", err)
	}
	if err := a.Source.RebindDevice(ctx, deviceID); err != nil {
		return fmt.Errorf("rollback rebind failed for %s: %w", deviceID, err)
	}
	return nil
}
