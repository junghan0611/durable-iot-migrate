package mock

import (
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateDevices(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))

	devices := GenerateDevices(100, rng)
	assert.Len(t, devices, 100)

	// IDs should be unique and sequential
	ids := make(map[string]bool)
	for _, d := range devices {
		assert.NotEmpty(t, d.ID)
		assert.NotEmpty(t, d.Name)
		assert.NotEmpty(t, d.Category)
		assert.NotEmpty(t, d.Protocol)
		assert.NotNil(t, d.SourceMeta)
		assert.False(t, ids[d.ID], "duplicate ID: %s", d.ID)
		ids[d.ID] = true
	}

	// Check variety (with 100 devices, we should see multiple categories/protocols)
	catSet := make(map[string]bool)
	protoSet := make(map[string]bool)
	for _, d := range devices {
		catSet[d.Category] = true
		protoSet[d.Protocol] = true
	}
	assert.Greater(t, len(catSet), 3, "should have variety in categories")
	assert.Greater(t, len(protoSet), 2, "should have variety in protocols")
}

func TestGenerateDevices_Deterministic(t *testing.T) {
	d1 := GenerateDevices(10, rand.New(rand.NewPCG(99, 0)))
	d2 := GenerateDevices(10, rand.New(rand.NewPCG(99, 0)))

	for i := range d1 {
		assert.Equal(t, d1[i].ID, d2[i].ID)
		assert.Equal(t, d1[i].Name, d2[i].Name)
		assert.Equal(t, d1[i].Category, d2[i].Category)
	}
}

func TestGenerateAutomations(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	devices := GenerateDevices(20, rng)
	autos := GenerateAutomations(50, devices, rng)

	assert.Len(t, autos, 50)

	for _, a := range autos {
		assert.NotEmpty(t, a.ID)
		assert.NotEmpty(t, a.Name)
		assert.NotEmpty(t, a.Triggers)
		assert.NotEmpty(t, a.Actions)
		assert.NotEmpty(t, a.DeviceRefs)
		assert.LessOrEqual(t, len(a.DeviceRefs), 3)

		// All device refs should point to valid devices
		deviceIDs := make(map[string]bool)
		for _, d := range devices {
			deviceIDs[d.ID] = true
		}
		for _, ref := range a.DeviceRefs {
			assert.True(t, deviceIDs[ref], "automation %s references unknown device %s", a.ID, ref)
		}
	}
}

func TestGenerateAutomations_SmallDevicePool(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 0))
	devices := GenerateDevices(1, rng) // Only 1 device
	autos := GenerateAutomations(5, devices, rng)

	assert.Len(t, autos, 5)
	for _, a := range autos {
		// All refs should point to the single device
		for _, ref := range a.DeviceRefs {
			assert.Equal(t, devices[0].ID, ref)
		}
	}
}
