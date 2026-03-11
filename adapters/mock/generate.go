package mock

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"

	"github.com/junghan0611/durable-iot-migrate/core/models"
)

var (
	categories = []string{"light", "sensor", "switch", "lock", "thermostat", "curtain", "camera", "doorbell", "plug", "fan"}
	protocols  = []string{"zigbee", "wifi", "thread", "matter", "ble"}
	rooms      = []string{"Living Room", "Bedroom", "Kitchen", "Bathroom", "Garage", "Office", "Hallway", "Garden"}
	triggerT   = []string{"device_state", "schedule", "sun", "webhook", "geofence"}
	conditionT = []string{"device_state", "time", "numeric", "zone"}
	actionT    = []string{"device_command", "notify", "delay", "scene", "webhook"}
)

// GenerateDevices creates n random devices with deterministic IDs.
func GenerateDevices(n int, rng *rand.Rand) []models.Device {
	devices := make([]models.Device, n)
	for i := range n {
		cat := categories[rng.IntN(len(categories))]
		room := rooms[rng.IntN(len(rooms))]
		devices[i] = models.Device{
			ID:       fmt.Sprintf("dev-%04d", i+1),
			Name:     fmt.Sprintf("%s %s", room, cat),
			Category: cat,
			Protocol: protocols[rng.IntN(len(protocols))],
			Online:   rng.Float64() > 0.05, // 95% online
			Properties: map[string]any{
				"firmware": fmt.Sprintf("v%d.%d.%d", rng.IntN(5)+1, rng.IntN(10), rng.IntN(20)),
				"rssi":     -30 - rng.IntN(50),
			},
			SourceMeta: json.RawMessage(`{"platform":"mock"}`),
		}
	}
	return devices
}

// GenerateAutomations creates m random automations referencing the given devices.
// Each automation references 1-3 devices randomly.
func GenerateAutomations(m int, devices []models.Device, rng *rand.Rand) []models.Automation {
	autos := make([]models.Automation, m)
	for i := range m {
		// Pick 1-3 random devices
		numRefs := rng.IntN(3) + 1
		if numRefs > len(devices) {
			numRefs = len(devices)
		}
		refs := make([]string, numRefs)
		perm := rng.Perm(len(devices))
		for j := range numRefs {
			refs[j] = devices[perm[j]].ID
		}

		triggers := make([]models.Trigger, rng.IntN(2)+1)
		for j := range triggers {
			triggers[j] = models.Trigger{
				Type:     triggerT[rng.IntN(len(triggerT))],
				DeviceID: refs[rng.IntN(len(refs))],
				Config:   map[string]any{"value": rng.IntN(100)},
			}
		}

		conditions := make([]models.Condition, rng.IntN(2))
		for j := range conditions {
			conditions[j] = models.Condition{
				Type:     conditionT[rng.IntN(len(conditionT))],
				DeviceID: refs[rng.IntN(len(refs))],
				Config:   map[string]any{"threshold": rng.IntN(50)},
			}
		}

		actions := make([]models.Action, rng.IntN(3)+1)
		for j := range actions {
			actions[j] = models.Action{
				Type:     actionT[rng.IntN(len(actionT))],
				DeviceID: refs[rng.IntN(len(refs))],
				Config:   map[string]any{"command": "toggle"},
			}
		}

		autos[i] = models.Automation{
			ID:         fmt.Sprintf("auto-%04d", i+1),
			Name:       fmt.Sprintf("Rule %d: %s → %s", i+1, triggers[0].Type, actions[0].Type),
			Enabled:    rng.Float64() > 0.1, // 90% enabled
			Triggers:   triggers,
			Conditions: conditions,
			Actions:    actions,
			DeviceRefs: refs,
			SourceMeta: json.RawMessage(`{"platform":"mock"}`),
		}
	}
	return autos
}
