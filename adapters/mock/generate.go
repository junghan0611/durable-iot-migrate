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
	regions    = []string{"kr", "us", "eu", "jp", "cn"}
	tiers      = []string{"free", "free", "free", "free", "premium", "premium", "enterprise"} // 57% free, 29% premium, 14% enterprise
	triggerT   = []string{"device_state", "schedule", "sun", "webhook", "geofence"}
	conditionT = []string{"device_state", "time", "numeric", "zone"}
	actionT    = []string{"device_command", "notify", "delay", "scene", "webhook"}
)

// Real-world device category distribution (approximate).
// Based on typical IoT platform analytics:
//   light/plug/switch: ~60%, sensor: ~15%, camera: ~10%, lock/doorbell: ~5%, thermostat: ~5%, other: ~5%
var categoryWeights = []struct {
	category string
	weight   int
}{
	{"light", 20}, {"plug", 20}, {"switch", 15},
	{"sensor", 15},
	{"camera", 10},
	{"thermostat", 5}, {"curtain", 3}, {"fan", 3},
	{"lock", 3}, {"doorbell", 3},
	{"smoke_detector", 2}, {"co_detector", 1},
}

var categoryWeightTotal int

func init() {
	for _, cw := range categoryWeights {
		categoryWeightTotal += cw.weight
	}
}

// weightedCategory picks a device category following realistic distribution.
func weightedCategory(rng *rand.Rand) string {
	r := rng.IntN(categoryWeightTotal)
	for _, cw := range categoryWeights {
		r -= cw.weight
		if r < 0 {
			return cw.category
		}
	}
	return "light"
}

// GenerateDevices creates n random devices with deterministic IDs.
// Uses realistic category distribution and assigns safety classes.
func GenerateDevices(n int, rng *rand.Rand) []models.Device {
	devices := make([]models.Device, n)
	for i := range n {
		cat := weightedCategory(rng)
		room := rooms[rng.IntN(len(rooms))]
		devices[i] = models.Device{
			ID:          fmt.Sprintf("dev-%04d", i+1),
			Name:        fmt.Sprintf("%s %s", room, cat),
			Category:    cat,
			Protocol:    protocols[rng.IntN(len(protocols))],
			Online:      rng.Float64() > 0.05, // 95% online
			SafetyClass: models.CategorySafetyClass(cat),
			Properties: map[string]any{
				"firmware": fmt.Sprintf("v%d.%d.%d", rng.IntN(5)+1, rng.IntN(10), rng.IntN(20)),
				"rssi":     -30 - rng.IntN(50),
			},
			SourceMeta: json.RawMessage(`{"platform":"mock"}`),
		}
	}
	return devices
}

// GenerateAccounts creates n accounts with realistic distributions.
func GenerateAccounts(n int, rng *rand.Rand) []models.Account {
	accounts := make([]models.Account, n)
	for i := range n {
		region := regions[rng.IntN(len(regions))]
		tier := tiers[rng.IntN(len(tiers))]

		// Device count follows power-law-ish distribution:
		// Most accounts: 1-5 devices, some: 5-20, few: 20-100
		var devCount int
		r := rng.Float64()
		switch {
		case r < 0.60: // 60% of accounts: 1-5 devices
			devCount = rng.IntN(5) + 1
		case r < 0.85: // 25%: 5-15 devices
			devCount = rng.IntN(11) + 5
		case r < 0.95: // 10%: 15-40 devices
			devCount = rng.IntN(26) + 15
		default: // 5%: 40-100 devices (enterprise/power users)
			devCount = rng.IntN(61) + 40
		}

		accounts[i] = models.Account{
			ID:       fmt.Sprintf("acct-%06d", i+1),
			Email:    fmt.Sprintf("user%06d@example.com", i+1),
			Region:   region,
			Tier:     tier,
			DevCount: devCount,
		}
	}
	return accounts
}

// GenerateAccountDevices creates devices for a given account.
// Respects realistic category distribution and sets AccountID.
func GenerateAccountDevices(account models.Account, rng *rand.Rand) []models.Device {
	devices := GenerateDevices(account.DevCount, rng)
	for i := range devices {
		devices[i].AccountID = account.ID
		devices[i].ID = fmt.Sprintf("%s-dev-%04d", account.ID, i+1)
	}
	return devices
}

// GenerateFleet creates a full fleet: accounts + devices + automations.
// Returns the total device and automation counts for verification.
type Fleet struct {
	Accounts    []models.Account
	Devices     map[string][]models.Device    // account_id → devices
	Automations map[string][]models.Automation // account_id → automations
	Stats       FleetStats
}

type FleetStats struct {
	TotalAccounts   int
	TotalDevices    int
	TotalAutomations int
	CriticalDevices int
	ImportantDevices int
	NormalDevices   int
	RegionBreakdown map[string]int
	CategoryBreakdown map[string]int
}

// GenerateFleet creates a realistic fleet of accounts, devices, and automations.
// autoRatio: average automations per device (e.g., 0.3 = 1 automation per ~3 devices).
func GenerateFleet(numAccounts int, autoRatio float64, rng *rand.Rand) *Fleet {
	fleet := &Fleet{
		Accounts:    GenerateAccounts(numAccounts, rng),
		Devices:     make(map[string][]models.Device),
		Automations: make(map[string][]models.Automation),
		Stats: FleetStats{
			TotalAccounts:     numAccounts,
			RegionBreakdown:   make(map[string]int),
			CategoryBreakdown: make(map[string]int),
		},
	}

	for _, acct := range fleet.Accounts {
		// Generate devices
		devices := GenerateAccountDevices(acct, rng)
		fleet.Devices[acct.ID] = devices
		fleet.Stats.TotalDevices += len(devices)
		fleet.Stats.RegionBreakdown[acct.Region] += len(devices)

		for _, d := range devices {
			fleet.Stats.CategoryBreakdown[d.Category]++
			switch d.SafetyClass {
			case models.SafetyCritical:
				fleet.Stats.CriticalDevices++
			case models.SafetyImportant:
				fleet.Stats.ImportantDevices++
			default:
				fleet.Stats.NormalDevices++
			}
		}

		// Generate automations
		numAutos := int(float64(len(devices)) * autoRatio)
		if numAutos > 0 {
			autos := GenerateAutomations(numAutos, devices, rng)
			for i := range autos {
				autos[i].ID = fmt.Sprintf("%s-%s", acct.ID, autos[i].ID)
			}
			fleet.Automations[acct.ID] = autos
			fleet.Stats.TotalAutomations += numAutos
		}
	}

	return fleet
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
