package mock

import (
	"math/rand/v2"
	"testing"

	"github.com/junghan0611/durable-iot-migrate/core/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAccounts_Distribution(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	accounts := GenerateAccounts(10000, rng)

	assert.Len(t, accounts, 10000)

	// Check region distribution (should be roughly uniform across 5 regions)
	regionCount := map[string]int{}
	tierCount := map[string]int{}
	totalDevices := 0
	maxDevices := 0

	for _, a := range accounts {
		regionCount[a.Region]++
		tierCount[a.Tier]++
		totalDevices += a.DevCount
		if a.DevCount > maxDevices {
			maxDevices = a.DevCount
		}
	}

	// Each region should have ~2000 ± 200 accounts (uniform 5 regions)
	for _, r := range regions {
		assert.InDelta(t, 2000, regionCount[r], 200, "region %s", r)
	}

	// Tier: ~57% free, ~29% premium, ~14% enterprise
	assert.InDelta(t, 5700, tierCount["free"], 300)
	assert.InDelta(t, 2900, tierCount["premium"], 300)
	assert.InDelta(t, 1400, tierCount["enterprise"], 300)

	// Average devices per account: weighted mean of [3, 10, 27.5, 70] ≈ ~8-10
	avgDev := float64(totalDevices) / float64(len(accounts))
	assert.InDelta(t, 9.0, avgDev, 2.0, "average devices per account")

	// Power users exist
	assert.Greater(t, maxDevices, 30, "should have power users with 30+ devices")

	t.Logf("10K accounts: avg %.1f dev/acct, total %d devices, max %d", avgDev, totalDevices, maxDevices)
}

func TestGenerateFleet_SmallScale(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	fleet := GenerateFleet(100, 0.3, rng)

	assert.Equal(t, 100, fleet.Stats.TotalAccounts)
	assert.Greater(t, fleet.Stats.TotalDevices, 0)
	assert.Greater(t, fleet.Stats.TotalAutomations, 0)

	// Safety class distribution: critical should be ~15% (camera 10% + lock 3% + doorbell 3%)
	criticalPct := float64(fleet.Stats.CriticalDevices) / float64(fleet.Stats.TotalDevices)
	assert.InDelta(t, 0.18, criticalPct, 0.08, "critical devices ~18%")

	t.Logf("Fleet: %d accounts, %d devices (%d critical, %d important, %d normal), %d automations",
		fleet.Stats.TotalAccounts, fleet.Stats.TotalDevices,
		fleet.Stats.CriticalDevices, fleet.Stats.ImportantDevices, fleet.Stats.NormalDevices,
		fleet.Stats.TotalAutomations)

	// Category distribution: lights+plugs+switches should dominate
	lightish := fleet.Stats.CategoryBreakdown["light"] + fleet.Stats.CategoryBreakdown["plug"] + fleet.Stats.CategoryBreakdown["switch"]
	lightPct := float64(lightish) / float64(fleet.Stats.TotalDevices)
	assert.Greater(t, lightPct, 0.35, "lights+plugs+switches should be >35%")
}

func TestGenerateFleet_10K_Accounts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 10K scale test in short mode")
	}

	rng := rand.New(rand.NewPCG(42, 0))
	fleet := GenerateFleet(10000, 0.3, rng)

	assert.Equal(t, 10000, fleet.Stats.TotalAccounts)

	// 10K accounts × ~8-10 devices = ~80K-100K devices
	assert.Greater(t, fleet.Stats.TotalDevices, 50000)
	assert.Less(t, fleet.Stats.TotalDevices, 200000)

	// Must have critical devices (cameras, locks)
	assert.Greater(t, fleet.Stats.CriticalDevices, 1000)

	// Every account should have at least 1 device
	for _, acct := range fleet.Accounts {
		devices := fleet.Devices[acct.ID]
		require.NotEmpty(t, devices, "account %s has no devices", acct.ID)

		// All devices should have correct AccountID
		for _, d := range devices {
			assert.Equal(t, acct.ID, d.AccountID)
			// Safety class should be assigned
			assert.NotEmpty(t, d.SafetyClass)
		}
	}

	// Region breakdown should exist
	assert.Len(t, fleet.Stats.RegionBreakdown, 5)

	t.Logf("10K fleet: %d devices (%d critical), %d automations, regions: %v",
		fleet.Stats.TotalDevices, fleet.Stats.CriticalDevices,
		fleet.Stats.TotalAutomations, fleet.Stats.RegionBreakdown)
}

func TestGenerateFleet_100K_Accounts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 100K scale test in short mode")
	}

	rng := rand.New(rand.NewPCG(42, 0))
	fleet := GenerateFleet(100000, 0.3, rng)

	// 100K accounts × ~9 avg = ~900K devices (approaching 1.4M user scale)
	assert.Greater(t, fleet.Stats.TotalDevices, 500000)
	assert.Greater(t, fleet.Stats.CriticalDevices, 10000)

	t.Logf("100K fleet: %d accounts, %d devices (%d critical, %d important, %d normal), %d automations",
		fleet.Stats.TotalAccounts, fleet.Stats.TotalDevices,
		fleet.Stats.CriticalDevices, fleet.Stats.ImportantDevices, fleet.Stats.NormalDevices,
		fleet.Stats.TotalAutomations)

	// Critical devices report for safety audit
	t.Logf("Safety audit: %.1f%% critical, %.1f%% important, %.1f%% normal",
		float64(fleet.Stats.CriticalDevices)/float64(fleet.Stats.TotalDevices)*100,
		float64(fleet.Stats.ImportantDevices)/float64(fleet.Stats.TotalDevices)*100,
		float64(fleet.Stats.NormalDevices)/float64(fleet.Stats.TotalDevices)*100)
}

func TestGenerateFleet_Deterministic(t *testing.T) {
	fleet1 := GenerateFleet(1000, 0.3, rand.New(rand.NewPCG(999, 0)))
	fleet2 := GenerateFleet(1000, 0.3, rand.New(rand.NewPCG(999, 0)))

	assert.Equal(t, fleet1.Stats.TotalDevices, fleet2.Stats.TotalDevices)
	assert.Equal(t, fleet1.Stats.TotalAutomations, fleet2.Stats.TotalAutomations)
	assert.Equal(t, fleet1.Stats.CriticalDevices, fleet2.Stats.CriticalDevices)

	// First account's first device should be identical
	acctID := fleet1.Accounts[0].ID
	assert.Equal(t, fleet1.Devices[acctID][0].ID, fleet2.Devices[acctID][0].ID)
	assert.Equal(t, fleet1.Devices[acctID][0].Category, fleet2.Devices[acctID][0].Category)
}

func TestGenerateAccountDevices_SafetyClassAssigned(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	acct := models.Account{ID: "test-acct", DevCount: 50}
	devices := GenerateAccountDevices(acct, rng)

	hasCritical := false
	for _, d := range devices {
		assert.NotEmpty(t, d.SafetyClass)
		assert.Equal(t, "test-acct", d.AccountID)
		assert.Equal(t, models.CategorySafetyClass(d.Category), d.SafetyClass)
		if d.SafetyClass == models.SafetyCritical {
			hasCritical = true
		}
	}

	// 50 devices should statistically have at least one critical device
	assert.True(t, hasCritical, "50 devices should have at least one critical device")
}
