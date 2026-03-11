package mock

import (
	"context"
	"math/rand/v2"
	"testing"

	"github.com/junghan0611/durable-iot-migrate/core/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSource_ListDevices(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	devices := GenerateDevices(5, rng)
	source := NewSource(devices, nil)

	listed, err := source.ListDevices(context.Background())
	require.NoError(t, err)
	assert.Len(t, listed, 5)
}

func TestSource_UnbindRebind(t *testing.T) {
	devices := []models.Device{{ID: "d1", Name: "Light"}}
	source := NewSource(devices, nil)
	ctx := context.Background()

	// Before unbind: listed
	listed, _ := source.ListDevices(ctx)
	assert.Len(t, listed, 1)

	// Unbind
	require.NoError(t, source.UnbindDevice(ctx, "d1"))
	assert.True(t, source.IsUnbound("d1"))

	// After unbind: not listed
	listed, _ = source.ListDevices(ctx)
	assert.Len(t, listed, 0)

	// Rebind
	require.NoError(t, source.RebindDevice(ctx, "d1"))
	assert.False(t, source.IsUnbound("d1"))

	// After rebind: listed again
	listed, _ = source.ListDevices(ctx)
	assert.Len(t, listed, 1)
}

func TestSource_UnbindNotFound(t *testing.T) {
	source := NewSource(nil, nil)
	err := source.UnbindDevice(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSource_FailUnbind(t *testing.T) {
	devices := []models.Device{{ID: "d1", Name: "Light"}}
	source := NewSource(devices, nil)
	source.FailUnbindIDs["d1"] = true

	err := source.UnbindDevice(context.Background(), "d1")
	assert.Error(t, err)
	assert.False(t, source.IsUnbound("d1"))
}

func TestSource_ListAutomations(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	devices := GenerateDevices(5, rng)
	autos := GenerateAutomations(3, devices, rng)
	source := NewSource(devices, autos)

	listed, err := source.ListAutomations(context.Background())
	require.NoError(t, err)
	assert.Len(t, listed, 3)
}

func TestTarget_BindVerifyUnbind(t *testing.T) {
	target := NewTarget()
	ctx := context.Background()
	device := models.Device{ID: "d1", Name: "Light"}

	// Bind
	require.NoError(t, target.BindDevice(ctx, device))
	assert.Len(t, target.BoundDevices(), 1)

	// Verify
	ok, err := target.VerifyDevice(ctx, "d1")
	require.NoError(t, err)
	assert.True(t, ok)

	// Unbind
	require.NoError(t, target.UnbindDevice(ctx, "d1"))
	assert.Empty(t, target.BoundDevices())

	// Verify after unbind
	ok, err = target.VerifyDevice(ctx, "d1")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestTarget_FailBind(t *testing.T) {
	target := NewTarget()
	target.FailDeviceIDs["d1"] = true

	err := target.BindDevice(context.Background(), models.Device{ID: "d1"})
	assert.Error(t, err)
	assert.Empty(t, target.BoundDevices())
}

func TestTarget_FailVerify(t *testing.T) {
	target := NewTarget()
	ctx := context.Background()

	// Bind first (so device exists)
	require.NoError(t, target.BindDevice(ctx, models.Device{ID: "d1"}))
	// But mark verification as failing
	target.FailVerifyIDs["d1"] = true

	ok, err := target.VerifyDevice(ctx, "d1")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestTarget_CreateDeleteAutomation(t *testing.T) {
	target := NewTarget()
	ctx := context.Background()

	auto := models.Automation{ID: "a1", Name: "Test Rule"}
	require.NoError(t, target.CreateAutomation(ctx, auto))
	assert.Len(t, target.BoundAutomations(), 1)

	require.NoError(t, target.DeleteAutomation(ctx, "a1"))
	assert.Empty(t, target.BoundAutomations())
}

func TestTarget_FailCreateAutomation(t *testing.T) {
	target := NewTarget()
	target.FailAutoIDs["a1"] = true

	err := target.CreateAutomation(context.Background(), models.Automation{ID: "a1"})
	assert.Error(t, err)
	assert.Empty(t, target.BoundAutomations())
}

func TestSource_DeviceCount(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	devices := GenerateDevices(7, rng)
	source := NewSource(devices, nil)
	assert.Equal(t, 7, source.DeviceCount())
}
