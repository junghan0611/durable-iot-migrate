package workflows_test

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/junghan0611/durable-iot-migrate/adapters/mock"
	"github.com/junghan0611/durable-iot-migrate/core/activities"
	"github.com/junghan0611/durable-iot-migrate/core/models"
	"github.com/junghan0611/durable-iot-migrate/core/workflows"
	mock2 "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

func registerAll(env *testsuite.TestWorkflowEnvironment, acts *activities.MigrationActivities) {
	env.RegisterActivity(acts.FetchDevices)
	env.RegisterActivity(acts.FetchAutomations)
	env.RegisterActivity(acts.MigrateDevice)
	env.RegisterActivity(acts.RollbackDevice)
}

// --- Phase 1: Device Migration Tests ---

func TestDeviceMigration_AllSuccess(t *testing.T) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()

	devices := []models.Device{
		{ID: "d1", Name: "Light", Category: "light", Protocol: "zigbee", Online: true},
		{ID: "d2", Name: "Sensor", Category: "sensor", Protocol: "zigbee", Online: true},
		{ID: "d3", Name: "Plug", Category: "switch", Protocol: "wifi", Online: true},
	}
	source := mock.NewSource(devices, nil)
	target := mock.NewTarget()
	acts := &activities.MigrationActivities{Source: source, Target: target}
	registerAll(env, acts)

	config := models.BatchConfig{
		BatchID: "test-all-success", SuccessThreshold: 0.95,
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
	assert.Len(t, target.BoundDevices(), 3)
}

func TestDeviceMigration_WithBindFailure(t *testing.T) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()

	devices := []models.Device{
		{ID: "d1", Name: "Light", Category: "light", Protocol: "zigbee"},
		{ID: "d2", Name: "Sensor", Category: "sensor", Protocol: "zigbee"},
		{ID: "d3", Name: "Plug", Category: "switch", Protocol: "wifi"},
	}
	source := mock.NewSource(devices, nil)
	target := mock.NewTarget()
	target.FailDeviceIDs["d2"] = true
	acts := &activities.MigrationActivities{Source: source, Target: target}
	registerAll(env, acts)

	config := models.BatchConfig{
		BatchID: "test-bind-fail", SuccessThreshold: 0.5,
	}
	env.ExecuteWorkflow(workflows.DeviceMigrationWorkflow, config)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result models.BatchResult
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.Succeeded)
	assert.Equal(t, 1, result.Failed)

	s := findStatus(result.Devices, "d2")
	require.NotNil(t, s)
	assert.Equal(t, "failed", s.Phase)
	assert.Contains(t, s.Error, "bind failed")
}

func TestDeviceMigration_WithVerifyFailure(t *testing.T) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()

	devices := []models.Device{
		{ID: "d1", Name: "Light", Category: "light", Protocol: "zigbee"},
		{ID: "d2", Name: "Sensor", Category: "sensor", Protocol: "zigbee"},
	}
	source := mock.NewSource(devices, nil)
	target := mock.NewTarget()
	target.FailVerifyIDs["d2"] = true
	acts := &activities.MigrationActivities{Source: source, Target: target}
	registerAll(env, acts)

	config := models.BatchConfig{
		BatchID: "test-verify-fail", SuccessThreshold: 0.4,
	}
	env.ExecuteWorkflow(workflows.DeviceMigrationWorkflow, config)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result models.BatchResult
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 1, result.Succeeded)
	assert.Equal(t, 1, result.Failed)

	s := findStatus(result.Devices, "d2")
	require.NotNil(t, s)
	assert.Contains(t, s.Error, "verify failed")

	// d2 should be compensated (back on source, not on target)
	assert.False(t, source.IsUnbound("d2"))
}

func TestDeviceMigration_WithUnbindFailure(t *testing.T) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()

	devices := []models.Device{
		{ID: "d1", Name: "Light", Category: "light", Protocol: "zigbee"},
		{ID: "d2", Name: "Sensor", Category: "sensor", Protocol: "zigbee"},
	}
	source := mock.NewSource(devices, nil)
	source.FailUnbindIDs["d1"] = true
	target := mock.NewTarget()
	acts := &activities.MigrationActivities{Source: source, Target: target}
	registerAll(env, acts)

	config := models.BatchConfig{
		BatchID: "test-unbind-fail", SuccessThreshold: 0.4,
	}
	env.ExecuteWorkflow(workflows.DeviceMigrationWorkflow, config)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result models.BatchResult
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 1, result.Succeeded)
	assert.Equal(t, 1, result.Failed)

	s := findStatus(result.Devices, "d1")
	require.NotNil(t, s)
	assert.Contains(t, s.Error, "unbind failed")
}

func TestDeviceMigration_ThresholdHalt(t *testing.T) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()

	devices := []models.Device{
		{ID: "d1", Name: "Light", Category: "light", Protocol: "zigbee"},
		{ID: "d2", Name: "Sensor", Category: "sensor", Protocol: "zigbee"},
	}
	source := mock.NewSource(devices, nil)
	target := mock.NewTarget()
	target.FailDeviceIDs["d2"] = true
	acts := &activities.MigrationActivities{Source: source, Target: target}
	registerAll(env, acts)

	config := models.BatchConfig{
		BatchID: "test-threshold", SuccessThreshold: 0.95,
	}
	env.ExecuteWorkflow(workflows.DeviceMigrationWorkflow, config)
	require.True(t, env.IsWorkflowCompleted())

	err := env.GetWorkflowError()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "below threshold")
}

func TestDeviceMigration_EmptyBatch(t *testing.T) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()

	source := mock.NewSource(nil, nil)
	target := mock.NewTarget()
	acts := &activities.MigrationActivities{Source: source, Target: target}
	registerAll(env, acts)

	config := models.BatchConfig{BatchID: "test-empty"}
	env.ExecuteWorkflow(workflows.DeviceMigrationWorkflow, config)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result models.BatchResult
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 0, result.TotalDevices)
	assert.Equal(t, 1.0, result.SuccessRate())
}

// --- DeviceIDs Filter ---

func TestDeviceMigration_FilterByDeviceIDs(t *testing.T) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()

	devices := []models.Device{
		{ID: "d1", Name: "Light", Category: "light", Protocol: "zigbee"},
		{ID: "d2", Name: "Sensor", Category: "sensor", Protocol: "zigbee"},
		{ID: "d3", Name: "Plug", Category: "switch", Protocol: "wifi"},
		{ID: "d4", Name: "Lock", Category: "lock", Protocol: "matter"},
	}
	source := mock.NewSource(devices, nil)
	target := mock.NewTarget()
	acts := &activities.MigrationActivities{Source: source, Target: target}
	registerAll(env, acts)

	config := models.BatchConfig{
		BatchID:   "test-filter",
		DeviceIDs: []string{"d2", "d4"}, // Only migrate 2 of 4
	}
	env.ExecuteWorkflow(workflows.DeviceMigrationWorkflow, config)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result models.BatchResult
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalDevices)
	assert.Equal(t, 2, result.Succeeded)
	assert.Len(t, target.BoundDevices(), 2)
}

// --- Phase 2: Automation Migration Tests ---

func TestDeviceMigration_WithAutomations(t *testing.T) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()

	devices := []models.Device{
		{ID: "d1", Name: "Light", Category: "light", Protocol: "zigbee"},
		{ID: "d2", Name: "Sensor", Category: "sensor", Protocol: "zigbee"},
	}
	autos := []models.Automation{
		{ID: "a1", Name: "Rule 1", DeviceRefs: []string{"d1"}},
		{ID: "a2", Name: "Rule 2", DeviceRefs: []string{"d1", "d2"}},
		{ID: "a3", Name: "Rule 3", DeviceRefs: []string{"d2"}},
	}
	source := mock.NewSource(devices, autos)
	target := mock.NewTarget()
	acts := &activities.MigrationActivities{Source: source, Target: target}
	registerAll(env, acts)

	config := models.BatchConfig{
		BatchID: "test-with-autos", MigrateAutos: true,
	}
	env.ExecuteWorkflow(workflows.DeviceMigrationWorkflow, config)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result models.BatchResult
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.Succeeded)
	assert.Equal(t, 3, result.TotalAutos)
	assert.Equal(t, 3, result.AutosSucceeded)
}

func TestDeviceMigration_AutosFilteredByFailedDevices(t *testing.T) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()

	devices := []models.Device{
		{ID: "d1", Name: "Light", Category: "light", Protocol: "zigbee"},
		{ID: "d2", Name: "Sensor", Category: "sensor", Protocol: "zigbee"},
	}
	autos := []models.Automation{
		{ID: "a1", Name: "Rule 1", DeviceRefs: []string{"d1"}},          // d1 ok → migrate
		{ID: "a2", Name: "Rule 2", DeviceRefs: []string{"d1", "d2"}},    // d2 failed → skip
		{ID: "a3", Name: "Rule 3", DeviceRefs: []string{"d2"}},          // d2 failed → skip
	}
	source := mock.NewSource(devices, autos)
	target := mock.NewTarget()
	target.FailDeviceIDs["d2"] = true
	acts := &activities.MigrationActivities{Source: source, Target: target}
	registerAll(env, acts)

	config := models.BatchConfig{
		BatchID: "test-auto-filter", MigrateAutos: true, SuccessThreshold: 0.4,
	}
	env.ExecuteWorkflow(workflows.DeviceMigrationWorkflow, config)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result models.BatchResult
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 1, result.Succeeded)
	assert.Equal(t, 1, result.Failed)
	// Only a1 should be migrated (only refs d1, which succeeded)
	assert.Equal(t, 1, result.TotalAutos)
	assert.Equal(t, 1, result.AutosSucceeded)
}

func TestDeviceMigration_AutosDisabled(t *testing.T) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()

	devices := []models.Device{
		{ID: "d1", Name: "Light", Category: "light", Protocol: "zigbee"},
	}
	autos := []models.Automation{
		{ID: "a1", Name: "Rule 1", DeviceRefs: []string{"d1"}},
	}
	source := mock.NewSource(devices, autos)
	target := mock.NewTarget()
	acts := &activities.MigrationActivities{Source: source, Target: target}
	registerAll(env, acts)

	config := models.BatchConfig{
		BatchID: "test-no-autos", MigrateAutos: false, // Phase 2 skipped
	}
	env.ExecuteWorkflow(workflows.DeviceMigrationWorkflow, config)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result models.BatchResult
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 0, result.TotalAutos)
}

// --- Scale Tests: Random devices + automations ---

func TestDeviceMigration_100Devices_50Automations(t *testing.T) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()

	rng := rand.New(rand.NewPCG(42, 0))
	devices := mock.GenerateDevices(100, rng)
	autos := mock.GenerateAutomations(50, devices, rng)

	source := mock.NewSource(devices, autos)
	target := mock.NewTarget()
	acts := &activities.MigrationActivities{Source: source, Target: target}
	registerAll(env, acts)

	config := models.BatchConfig{
		BatchID: "test-scale-100", MigrateAutos: true, SuccessThreshold: 0.95,
	}
	env.ExecuteWorkflow(workflows.DeviceMigrationWorkflow, config)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result models.BatchResult
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 100, result.TotalDevices)
	assert.Equal(t, 100, result.Succeeded)
	assert.Equal(t, 0, result.Failed)
	assert.Equal(t, 50, result.TotalAutos)
	assert.Len(t, target.BoundDevices(), 100)
}

func TestDeviceMigration_500Devices_RandomFailures(t *testing.T) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()

	rng := rand.New(rand.NewPCG(7, 0))
	devices := mock.GenerateDevices(500, rng)
	autos := mock.GenerateAutomations(200, devices, rng)

	source := mock.NewSource(devices, autos)
	target := mock.NewTarget()

	// Inject ~2% random failures (10 of 500)
	failRng := rand.New(rand.NewPCG(99, 0))
	perm := failRng.Perm(500)
	for i := range 10 {
		target.FailDeviceIDs[devices[perm[i]].ID] = true
	}

	acts := &activities.MigrationActivities{Source: source, Target: target}
	registerAll(env, acts)

	config := models.BatchConfig{
		BatchID: "test-scale-500", MigrateAutos: true, SuccessThreshold: 0.95,
	}
	env.ExecuteWorkflow(workflows.DeviceMigrationWorkflow, config)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result models.BatchResult
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 500, result.TotalDevices)
	assert.Equal(t, 490, result.Succeeded)
	assert.Equal(t, 10, result.Failed)
	assert.InDelta(t, 0.98, result.SuccessRate(), 0.01)

	// Failed devices should have been compensated (back on source)
	for i := range 10 {
		assert.False(t, source.IsUnbound(devices[perm[i]].ID))
	}

	// Automations referencing only successful devices should be counted
	assert.Greater(t, result.TotalAutos, 0)
}

func TestDeviceMigration_DefaultConcurrency(t *testing.T) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()

	devices := []models.Device{{ID: "d1", Name: "Light", Category: "light", Protocol: "zigbee"}}
	source := mock.NewSource(devices, nil)
	target := mock.NewTarget()
	acts := &activities.MigrationActivities{Source: source, Target: target}
	registerAll(env, acts)

	// No Concurrency or SuccessThreshold set — should use defaults
	config := models.BatchConfig{BatchID: "test-defaults"}
	env.ExecuteWorkflow(workflows.DeviceMigrationWorkflow, config)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result models.BatchResult
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 1, result.Succeeded)
}

// --- Edge Cases ---

func TestDeviceMigration_FetchDevicesFails(t *testing.T) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()

	source := mock.NewSource(nil, nil)
	target := mock.NewTarget()
	acts := &activities.MigrationActivities{Source: source, Target: target}
	registerAll(env, acts)

	// Override FetchDevices to fail
	env.OnActivity(acts.FetchDevices, mock2.Anything).Return(nil, fmt.Errorf("connection refused"))

	config := models.BatchConfig{BatchID: "test-fetch-fail"}
	env.ExecuteWorkflow(workflows.DeviceMigrationWorkflow, config)
	require.True(t, env.IsWorkflowCompleted())

	err := env.GetWorkflowError()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetch devices failed")
}

func TestDeviceMigration_FetchAutomationsFails(t *testing.T) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()

	devices := []models.Device{
		{ID: "d1", Name: "Light", Category: "light", Protocol: "zigbee"},
	}
	source := mock.NewSource(devices, nil)
	target := mock.NewTarget()
	acts := &activities.MigrationActivities{Source: source, Target: target}
	registerAll(env, acts)

	// Override FetchAutomations to fail
	env.OnActivity(acts.FetchAutomations, mock2.Anything).Return(nil, fmt.Errorf("automation service down"))

	config := models.BatchConfig{
		BatchID: "test-fetch-auto-fail", MigrateAutos: true,
	}
	env.ExecuteWorkflow(workflows.DeviceMigrationWorkflow, config)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError()) // Should succeed (phase 2 is best-effort)

	var result models.BatchResult
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 1, result.Succeeded)
	assert.Equal(t, 0, result.TotalAutos) // Automations skipped
}

func TestDeviceMigration_ActivityError(t *testing.T) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()

	devices := []models.Device{
		{ID: "d1", Name: "Light", Category: "light", Protocol: "zigbee"},
		{ID: "d2", Name: "Sensor", Category: "sensor", Protocol: "zigbee"},
	}
	source := mock.NewSource(devices, nil)
	target := mock.NewTarget()
	acts := &activities.MigrationActivities{Source: source, Target: target}
	registerAll(env, acts)

	// Override MigrateDevice to return an activity-level error (not a status error)
	env.OnActivity(acts.MigrateDevice, mock2.Anything, mock2.Anything).Return(
		models.MigrationStatus{}, fmt.Errorf("worker crashed"),
	)

	config := models.BatchConfig{
		BatchID: "test-activity-error", SuccessThreshold: 0.01,
	}
	env.ExecuteWorkflow(workflows.DeviceMigrationWorkflow, config)
	require.True(t, env.IsWorkflowCompleted())

	// With 0% success, even 0.01 threshold is breached → error
	err := env.GetWorkflowError()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "below threshold")
}

// Verify that unused mock import is used
var _ = fmt.Sprintf
var _ = mock2.Anything

// --- Helpers ---

func findStatus(statuses []models.MigrationStatus, deviceID string) *models.MigrationStatus {
	for _, s := range statuses {
		if s.DeviceID == deviceID {
			return &s
		}
	}
	return nil
}
