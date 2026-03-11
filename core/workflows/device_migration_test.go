package workflows_test

import (
	"testing"

	"github.com/junghan0611/durable-iot-migrate/adapters/mock"
	"github.com/junghan0611/durable-iot-migrate/core/activities"
	"github.com/junghan0611/durable-iot-migrate/core/models"
	"github.com/junghan0611/durable-iot-migrate/core/workflows"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

func TestDeviceMigration_AllSuccess(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	devices := []models.Device{
		{ID: "d1", Name: "Light", Category: "light", Protocol: "zigbee", Online: true},
		{ID: "d2", Name: "Sensor", Category: "sensor", Protocol: "zigbee", Online: true},
		{ID: "d3", Name: "Plug", Category: "switch", Protocol: "wifi", Online: true},
	}

	source := mock.NewSource(devices, nil)
	target := mock.NewTarget()
	acts := &activities.MigrationActivities{Source: source, Target: target}

	env.RegisterActivity(acts.FetchDevices)
	env.RegisterActivity(acts.FetchAutomations)
	env.RegisterActivity(acts.MigrateDevice)
	env.RegisterActivity(acts.RollbackDevice)

	config := models.BatchConfig{
		BatchID:          "test-batch-001",
		SourcePlatform:   "mock",
		TargetPlatform:   "mock",
		Concurrency:      5,
		SuccessThreshold: 0.95,
	}

	env.ExecuteWorkflow(workflows.DeviceMigrationWorkflow, config)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result models.BatchResult
	require.NoError(t, env.GetWorkflowResult(&result))

	assert.Equal(t, 3, result.TotalDevices)
	assert.Equal(t, 3, result.Succeeded)
	assert.Equal(t, 0, result.Failed)
	assert.Equal(t, 1.0, result.SuccessRate())

	// Verify all devices are on the target
	assert.Len(t, target.BoundDevices(), 3)
}

func TestDeviceMigration_WithFailures(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	devices := []models.Device{
		{ID: "d1", Name: "Light", Category: "light", Protocol: "zigbee", Online: true},
		{ID: "d2", Name: "Sensor", Category: "sensor", Protocol: "zigbee", Online: true},
		{ID: "d3", Name: "Plug", Category: "switch", Protocol: "wifi", Online: true},
	}

	source := mock.NewSource(devices, nil)
	target := mock.NewTarget()
	target.FailDeviceIDs["d2"] = true // d2 will fail to bind

	acts := &activities.MigrationActivities{Source: source, Target: target}

	env.RegisterActivity(acts.FetchDevices)
	env.RegisterActivity(acts.FetchAutomations)
	env.RegisterActivity(acts.MigrateDevice)
	env.RegisterActivity(acts.RollbackDevice)

	config := models.BatchConfig{
		BatchID:          "test-batch-002",
		SourcePlatform:   "mock",
		TargetPlatform:   "mock",
		Concurrency:      5,
		SuccessThreshold: 0.5, // Low threshold so workflow completes
	}

	env.ExecuteWorkflow(workflows.DeviceMigrationWorkflow, config)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result models.BatchResult
	require.NoError(t, env.GetWorkflowResult(&result))

	assert.Equal(t, 3, result.TotalDevices)
	assert.Equal(t, 2, result.Succeeded)
	assert.Equal(t, 1, result.Failed)

	// Failed device should have been rolled back (rebound to source)
	failedStatus := findStatus(result.Devices, "d2")
	require.NotNil(t, failedStatus)
	assert.Equal(t, "failed", failedStatus.Phase)
	assert.Contains(t, failedStatus.Error, "bind failed")
}

func TestDeviceMigration_ThresholdHalt(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	devices := []models.Device{
		{ID: "d1", Name: "Light", Category: "light", Protocol: "zigbee", Online: true},
		{ID: "d2", Name: "Sensor", Category: "sensor", Protocol: "zigbee", Online: true},
	}

	source := mock.NewSource(devices, nil)
	target := mock.NewTarget()
	target.FailDeviceIDs["d2"] = true // 50% failure

	acts := &activities.MigrationActivities{Source: source, Target: target}

	env.RegisterActivity(acts.FetchDevices)
	env.RegisterActivity(acts.FetchAutomations)
	env.RegisterActivity(acts.MigrateDevice)
	env.RegisterActivity(acts.RollbackDevice)

	config := models.BatchConfig{
		BatchID:          "test-batch-003",
		SourcePlatform:   "mock",
		TargetPlatform:   "mock",
		Concurrency:      5,
		SuccessThreshold: 0.95, // High threshold — 50% success should halt
	}

	env.ExecuteWorkflow(workflows.DeviceMigrationWorkflow, config)
	require.True(t, env.IsWorkflowCompleted())

	// Should have error due to threshold breach
	err := env.GetWorkflowError()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "below threshold")
}

func findStatus(statuses []models.MigrationStatus, deviceID string) *models.MigrationStatus {
	for _, s := range statuses {
		if s.DeviceID == deviceID {
			return &s
		}
	}
	return nil
}
