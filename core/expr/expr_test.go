package expr

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ═══════════════════════════════════════════════════════════════
// The core thesis: the SAME automation rule, expressed by different
// platforms, should produce structurally equivalent Expr trees.
//
// "Temperature above 25°C → turn on AC"
//
// Tuya:  conditions[0].entity_type=1, dp_id="4", comparator=">", value=25
// HA:    trigger: numeric_state, entity_id: sensor.temp, above: 25
// ST:    if.greaterThan(device.main.temperatureMeasurement.temperature, 25)
// Homey: trigger.uri: homey:device:sensor, id: measure_temperature
//
// All map to: Gt(State("sensor", "temperature"), Lit(25))
// ═══════════════════════════════════════════════════════════════

func TestCrossPlatformEquivalence_TemperatureThreshold(t *testing.T) {
	// Tuya representation: dp_id "4" is temperature, comparator ">", value 25
	tuya := GtExpr(
		State("tuya-device-001", "temperature"),
		Lit(25),
	)

	// HA representation: sensor.temperature above 25
	ha := GtExpr(
		State("sensor.living_room_temp", "temperature"),
		Lit(25),
	)

	// SmartThings representation
	st := GtExpr(
		StateWithComponent("st-device-001", "main", "temperature"),
		Lit(25),
	)

	// All three are structurally equivalent
	assert.True(t, StructuralEquiv(tuya, ha), "Tuya ≡ HA (structure)")
	assert.True(t, StructuralEquiv(ha, st), "HA ≡ ST (structure)")

	// Semantic equivalence (attribute matches, device IDs differ)
	assert.True(t, Equiv(tuya, ha), "Tuya ≡ HA (semantic)")
	assert.True(t, Equiv(ha, st), "HA ≡ ST (semantic)")
}

func TestCrossPlatformEquivalence_MotionLight(t *testing.T) {
	// "When motion detected AND light is off → turn on light, wait 5min, turn off"
	//
	// This is the most common IoT automation pattern across ALL platforms.

	// The trigger: motion == true
	motionTrigger := EqExpr(State("sensor-01", "motion"), Lit(true))

	// The condition: light == off
	lightOff := EqExpr(State("light-01", "state"), Lit("off"))

	// The action sequence: [turn_on, delay 300s, turn_off]
	actions := SeqExpr(
		Cmd("light-01", "state", "on"),
		DelayExpr(300),
		Cmd("light-01", "state", "off"),
	)

	// Full automation expression
	auto := AndExpr(motionTrigger, lightOff)

	assert.True(t, IsValid(motionTrigger))
	assert.True(t, IsValid(lightOff))
	assert.True(t, IsValid(actions))
	assert.True(t, IsValid(auto))

	// Device refs
	refs := DeviceRefs(auto)
	assert.Contains(t, refs, "sensor-01")
	assert.Contains(t, refs, "light-01")

	actionRefs := DeviceRefs(actions)
	assert.Contains(t, actionRefs, "light-01")
	assert.Len(t, actionRefs, 1) // Only light-01
}

func TestCombinator_CompoundConditions(t *testing.T) {
	// "(temp > 25 OR humidity > 80) AND time_between(22:00, 06:00)"
	tempHigh := GtExpr(State("sensor-01", "temperature"), Lit(25))
	humidHigh := GtExpr(State("sensor-01", "humidity"), Lit(80))

	timeCond := BetweenExpr(Time("now"), Lit("22:00"), Lit("06:00"))

	full := AndExpr(OrExpr(tempHigh, humidHigh), timeCond)

	assert.True(t, IsValid(full))
	assert.Equal(t, 4, Depth(full))    // And → Or → Gt → StateRef
	assert.Equal(t, 12, NodeCount(full)) // And(1) + Or(1)+Gt(3)+Gt(3) + Between(1)+Time(1)+Lit(1)+Lit(1)

	refs := DeviceRefs(full)
	assert.Len(t, refs, 1) // Only sensor-01
	assert.Contains(t, refs, "sensor-01")
}

func TestNot_InvertedCondition(t *testing.T) {
	// Homey's "inverted: true" on a condition
	// Light is NOT on → NotExpr(EqExpr(State, Lit("on")))
	lightOn := EqExpr(State("light-01", "state"), Lit("on"))
	lightNotOn := NotExpr(lightOn)

	assert.True(t, IsValid(lightNotOn))
	assert.False(t, Equiv(lightOn, lightNotOn))

	// Not(Not(x)) is structurally deeper but same idea
	doubleNot := NotExpr(NotExpr(lightOn))
	assert.True(t, IsValid(doubleNot))
	assert.Equal(t, 4, Depth(doubleNot))
}

func TestSeq_ActionSequence(t *testing.T) {
	// Turn on → delay 5s → set brightness 80% → notify
	actions := SeqExpr(
		Cmd("light-01", "state", "on"),
		DelayExpr(5),
		Cmd("light-01", "brightness", 80).WithMeta("service", "light.turn_on"),
		NotifyExpr("Light is on!"),
	)

	assert.True(t, IsValid(actions))
	assert.Equal(t, 4, len(actions.Children))

	// Seq is order-dependent
	reversed := SeqExpr(
		NotifyExpr("Light is on!"),
		Cmd("light-01", "brightness", 80),
		DelayExpr(5),
		Cmd("light-01", "state", "on"),
	)
	assert.False(t, Equiv(actions, reversed), "Seq order matters")
}

func TestParallel_OrderIndependent(t *testing.T) {
	a := Cmd("light-01", "state", "on")
	b := Cmd("light-02", "state", "on")
	c := Cmd("light-03", "state", "on")

	p1 := ParallelExpr(a, b, c)
	p2 := ParallelExpr(c, a, b) // Different order

	assert.True(t, Equiv(p1, p2), "Parallel is order-independent")
}

func TestValidation_Errors(t *testing.T) {
	tests := []struct {
		name string
		expr *Expr
		ok   bool
	}{
		{"valid eq", EqExpr(State("d", "s"), Lit(1)), true},
		{"eq needs 2 children", &Expr{Op: Eq, Children: []*Expr{Lit(1)}}, false},
		{"between needs 3", BetweenExpr(Lit(1), Lit(2), Lit(3)), true},
		{"between with 2", &Expr{Op: Between, Children: []*Expr{Lit(1), Lit(2)}}, false},
		{"not needs 1", NotExpr(Lit(true)), true},
		{"not with 2", &Expr{Op: Not, Children: []*Expr{Lit(1), Lit(2)}}, false},
		{"and needs 2+", AndExpr(Lit(1), Lit(2)), true},
		{"and with 1", &Expr{Op: And, Children: []*Expr{Lit(1)}}, false},
		{"state_ref without ref", &Expr{Op: StateRef}, false},
		{"state_ref without deviceID", &Expr{Op: StateRef, Ref: &DeviceRef{Attribute: "temp"}}, false},
		{"delay without value", &Expr{Op: Delay}, false},
		{"command without ref", &Expr{Op: Command, Value: "on"}, false},
		{"literal is leaf", Lit(42), true},
		{"literal with children", &Expr{Op: Literal, Value: 42, Children: []*Expr{Lit(1)}}, false},
		{"in needs 2+", InExpr(Lit(1), Lit(2), Lit(3)), true},
		{"in needs at least 2", &Expr{Op: In, Children: []*Expr{Lit(1)}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Validate(tt.expr)
			if tt.ok {
				assert.Empty(t, errs, "expected valid")
			} else {
				assert.NotEmpty(t, errs, "expected errors")
			}
		})
	}
}

func TestEquiv_AndIsOrderIndependent(t *testing.T) {
	a := EqExpr(State("d1", "temp"), Lit(25))
	b := EqExpr(State("d2", "humidity"), Lit(60))

	ab := AndExpr(a, b)
	ba := AndExpr(b, a)

	assert.True(t, Equiv(ab, ba), "AND is commutative")
}

func TestEquiv_DifferentOpsNotEqual(t *testing.T) {
	a := GtExpr(State("d", "temp"), Lit(25))
	b := LtExpr(State("d", "temp"), Lit(25))

	assert.False(t, Equiv(a, b))
}

func TestEquivWithMapping(t *testing.T) {
	// Tuya uses true/false for on/off, HA uses "on"/"off"
	tuya := EqExpr(State("device-01", "switch"), Lit(true))
	ha := EqExpr(State("switch.lamp", "switch"), Lit("on"))

	// Without mapping: not equivalent (true != "on")
	assert.False(t, Equiv(tuya, ha))

	// With mapping: true → "on", false → "off"
	mapper := func(ref *DeviceRef, value any) any {
		switch v := value.(type) {
		case bool:
			if v {
				return "on"
			}
			return "off"
		}
		return value
	}

	assert.True(t, EquivWithMapping(tuya, ha, mapper), "with mapper: true == on")
}

func TestJSON_RoundTrip(t *testing.T) {
	// Complex expression should survive JSON round-trip
	expr := AndExpr(
		GtExpr(State("sensor-01", "temperature"), Lit(25)),
		NotExpr(EqExpr(State("light-01", "state"), Lit("on"))),
	)

	data, err := json.Marshal(expr)
	require.NoError(t, err)

	var restored Expr
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, And, restored.Op)
	assert.Len(t, restored.Children, 2)
	assert.Equal(t, Gt, restored.Children[0].Op)
	assert.Equal(t, Not, restored.Children[1].Op)

	t.Logf("JSON: %s", string(data))
}

func TestMeta_PlatformHints(t *testing.T) {
	// Meta preserves platform-specific info without polluting the tree
	cmd := Cmd("light-01", "state", "on").
		WithMeta("ha_service", "light.turn_on").
		WithMeta("tuya_dp_id", "1").
		WithMeta("st_capability", "switch")

	assert.True(t, IsValid(cmd))
	assert.Equal(t, "light.turn_on", cmd.Meta["ha_service"])
	assert.Equal(t, "1", cmd.Meta["tuya_dp_id"])
	assert.Equal(t, "switch", cmd.Meta["st_capability"])
}

func TestSceneExpr(t *testing.T) {
	scene := SceneExpr("night-mode-001")
	assert.True(t, IsValid(scene))
	assert.True(t, scene.IsAction())
	assert.Equal(t, "night-mode-001", scene.Value)
}

func TestDepthAndNodeCount(t *testing.T) {
	// Simple: Eq(State, Lit) → depth 2, nodes 3
	simple := EqExpr(State("d", "s"), Lit(1))
	assert.Equal(t, 2, Depth(simple))
	assert.Equal(t, 3, NodeCount(simple))

	// Nested: And(Eq(State,Lit), Or(Gt(State,Lit), Lt(State,Lit)))
	nested := AndExpr(
		EqExpr(State("d", "s"), Lit(1)),
		OrExpr(
			GtExpr(State("d", "t"), Lit(2)),
			LtExpr(State("d", "u"), Lit(3)),
		),
	)
	assert.Equal(t, 4, Depth(nested))
	assert.Equal(t, 11, NodeCount(nested))
}
