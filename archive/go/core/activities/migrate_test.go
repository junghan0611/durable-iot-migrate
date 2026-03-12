package activities_test

import (
	"context"
	"math/rand/v2"
	"testing"

	"github.com/junghan0611/durable-iot-migrate/adapters/mock"
	"github.com/junghan0611/durable-iot-migrate/core/activities"
	"github.com/junghan0611/durable-iot-migrate/core/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

func newTestEnv(t *testing.T) (*testsuite.TestActivityEnvironment, *activities.MigrationActivities, *mock.Source, *mock.Target) {
	rng := rand.New(rand.NewPCG(42, 0))
	devices := mock.GenerateDevices(10, rng)
	autos := mock.GenerateAutomations(5, devices, rng)

	source := mock.NewSource(devices, autos)
	target := mock.NewTarget()
	acts := &activities.MigrationActivities{Source: source, Target: target}

	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(acts.FetchDevices)
	env.RegisterActivity(acts.FetchAutomations)
	env.RegisterActivity(acts.MigrateDevice)
	env.RegisterActivity(acts.RollbackDevice)

	return env, acts, source, target
}

func TestFetchDevices(t *testing.T) {
	env, _, _, _ := newTestEnv(t)

	val, err := env.ExecuteActivity("FetchDevices")
	require.NoError(t, err)

	var devices []models.Device
	require.NoError(t, val.Get(&devices))
	assert.Len(t, devices, 10)

	// All should have valid IDs and categories
	for _, d := range devices {
		assert.NotEmpty(t, d.ID)
		assert.NotEmpty(t, d.Name)
		assert.NotEmpty(t, d.Category)
		assert.NotEmpty(t, d.Protocol)
	}
}

func TestFetchAutomations(t *testing.T) {
	env, _, _, _ := newTestEnv(t)

	val, err := env.ExecuteActivity("FetchAutomations")
	require.NoError(t, err)

	var autos []models.Automation
	require.NoError(t, val.Get(&autos))
	assert.Len(t, autos, 5)

	for _, a := range autos {
		assert.NotEmpty(t, a.ID)
		assert.NotEmpty(t, a.Name)
		assert.NotEmpty(t, a.Triggers)
		assert.NotEmpty(t, a.Actions)
		assert.NotEmpty(t, a.DeviceRefs)
	}
}

func TestMigrateDevice_Success(t *testing.T) {
	env, _, source, target := newTestEnv(t)

	device := models.Device{
		ID: "dev-0001", Name: "Living Room light", Category: "light", Protocol: "zigbee", Online: true,
	}

	val, err := env.ExecuteActivity("MigrateDevice", device)
	require.NoError(t, err)

	var status models.MigrationStatus
	require.NoError(t, val.Get(&status))

	assert.Equal(t, "complete", status.Phase)
	assert.Equal(t, "dev-0001", status.DeviceID)
	assert.Empty(t, status.Error)

	// Source should have unbound the device
	assert.True(t, source.IsUnbound("dev-0001"))
	// Target should have the device
	assert.Len(t, target.BoundDevices(), 1)
}

func TestMigrateDevice_UnbindFails(t *testing.T) {
	env, _, source, target := newTestEnv(t)
	source.FailUnbindIDs["dev-0001"] = true

	device := models.Device{ID: "dev-0001", Name: "Test", Category: "light", Protocol: "zigbee"}

	val, err := env.ExecuteActivity("MigrateDevice", device)
	require.NoError(t, err) // Activity doesn't return error, returns status

	var status models.MigrationStatus
	require.NoError(t, val.Get(&status))

	assert.Equal(t, "failed", status.Phase)
	assert.Contains(t, status.Error, "unbind failed")

	// Source should NOT have unbound
	assert.False(t, source.IsUnbound("dev-0001"))
	// Target should be empty
	assert.Empty(t, target.BoundDevices())
}

func TestMigrateDevice_BindFails_Compensates(t *testing.T) {
	env, _, source, target := newTestEnv(t)
	target.FailDeviceIDs["dev-0001"] = true

	device := models.Device{ID: "dev-0001", Name: "Test", Category: "light", Protocol: "zigbee"}

	val, err := env.ExecuteActivity("MigrateDevice", device)
	require.NoError(t, err)

	var status models.MigrationStatus
	require.NoError(t, val.Get(&status))

	assert.Equal(t, "failed", status.Phase)
	assert.Contains(t, status.Error, "bind failed")

	// Compensation: device should be rebound to source
	assert.False(t, source.IsUnbound("dev-0001"))
	// Target should be empty
	assert.Empty(t, target.BoundDevices())
}

func TestMigrateDevice_VerifyFails_Compensates(t *testing.T) {
	env, _, source, target := newTestEnv(t)
	target.FailVerifyIDs["dev-0001"] = true

	device := models.Device{ID: "dev-0001", Name: "Test", Category: "light", Protocol: "zigbee"}

	val, err := env.ExecuteActivity("MigrateDevice", device)
	require.NoError(t, err)

	var status models.MigrationStatus
	require.NoError(t, val.Get(&status))

	assert.Equal(t, "failed", status.Phase)
	assert.Contains(t, status.Error, "verify failed")

	// Compensation: unbound from target + rebound to source
	assert.False(t, source.IsUnbound("dev-0001"))
	assert.Empty(t, target.BoundDevices())
}

func TestRollbackDevice(t *testing.T) {
	env, _, source, target := newTestEnv(t)

	// First migrate successfully
	device := models.Device{ID: "dev-0001", Name: "Test", Category: "light", Protocol: "zigbee"}
	_, err := env.ExecuteActivity("MigrateDevice", device)
	require.NoError(t, err)
	assert.Len(t, target.BoundDevices(), 1)

	// Now rollback
	_, err = env.ExecuteActivity("RollbackDevice", "dev-0001")
	require.NoError(t, err)

	// Device should be back on source, gone from target
	assert.False(t, source.IsUnbound("dev-0001"))
	assert.Empty(t, target.BoundDevices())
}

func TestRollbackDevice_RebindFails(t *testing.T) {
	env, _, source, target := newTestEnv(t)

	// Migrate successfully
	device := models.Device{ID: "dev-0001", Name: "Test", Category: "light", Protocol: "zigbee"}
	_, err := env.ExecuteActivity("MigrateDevice", device)
	require.NoError(t, err)

	// Make rebind fail during rollback
	source.FailUnbindIDs["dev-0001"] = true // This won't affect RebindDevice
	// We need to test rebind failure — remove the device from source so rebind path hits
	// Actually RebindDevice just deletes from unbound map, it doesn't fail
	// So let's verify the target unbind path works even when target unbind "fails" (device already removed)
	_ = target.UnbindDevice(context.Background(), "dev-0001") // Pre-remove

	// Rollback should still succeed (target unbind is best-effort)
	_, err = env.ExecuteActivity("RollbackDevice", "dev-0001")
	require.NoError(t, err)
}

func TestMigrateDevice_VerifyReturnsError(t *testing.T) {
	// Test the err != nil path in verify (distinct from ok == false)
	env, _, source, target := newTestEnv(t)
	target.ErrorVerifyIDs = map[string]bool{"dev-0001": true}

	device := models.Device{ID: "dev-0001", Name: "Test", Category: "light", Protocol: "zigbee"}
	val, err := env.ExecuteActivity("MigrateDevice", device)
	require.NoError(t, err)

	var status models.MigrationStatus
	require.NoError(t, val.Get(&status))
	assert.Equal(t, "failed", status.Phase)
	assert.Contains(t, status.Error, "verify failed")

	// Compensation: device back on source
	assert.False(t, source.IsUnbound("dev-0001"))
	assert.Empty(t, target.BoundDevices())
}

func TestMigrateDevice_UnknownDevice(t *testing.T) {
	env, _, _, _ := newTestEnv(t)

	device := models.Device{ID: "unknown-device", Name: "Ghost", Category: "sensor", Protocol: "wifi"}
	val, err := env.ExecuteActivity("MigrateDevice", device)
	require.NoError(t, err)

	var status models.MigrationStatus
	require.NoError(t, val.Get(&status))
	assert.Equal(t, "failed", status.Phase)
	assert.Contains(t, status.Error, "unbind failed")
}

func TestRollbackDevice_RebindFails_ReturnsError(t *testing.T) {
	env, _, source, target := newTestEnv(t)

	// Migrate first
	device := models.Device{ID: "dev-0001", Name: "Test", Category: "light", Protocol: "zigbee"}
	_, err := env.ExecuteActivity("MigrateDevice", device)
	require.NoError(t, err)
	assert.Len(t, target.BoundDevices(), 1)

	// Make rebind fail
	source.FailRebindIDs["dev-0001"] = true

	// Rollback should return error (rebind failure is not best-effort)
	_, err = env.ExecuteActivity("RollbackDevice", "dev-0001")
	require.Error(t, err)
}
