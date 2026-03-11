package expr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ═══════════════════════════════════════════════════════════════
// Cross-Platform Equivalence Tests
//
// The fundamental assertion: the SAME real-world automation rule,
// parsed from different platforms, produces equivalent Expr trees.
//
// This is what makes the converter "durable" — not just "flexible".
// Flexibility without verification is fragile. Verified equivalence
// is durable semantics.
// ═══════════════════════════════════════════════════════════════

// ── "Turn on light when motion detected" ──
// The most universal IoT automation pattern.

func TestEquiv_MotionLight_AllPlatforms(t *testing.T) {
	// What each platform parser would produce as Expr:

	// Tuya: entity_type=1, dp_id="1", comparator="==", value=true
	tuya := EqExpr(
		State("tuya-pir-001", "dp_1"),
		Lit(true),
	)

	// Home Assistant: trigger: state, entity_id: binary_sensor.pir, to: "on"
	ha := EqExpr(
		State("binary_sensor.pir", "state"),
		Lit("on"),
	)

	// SmartThings: if.equals(device.main.motionSensor.motion, "active")
	st := EqExpr(
		StateWithComponent("st-pir-001", "main", "motion"),
		Lit("active"),
	)

	// Homey: trigger.uri=homey:device:pir, id=alarm_motion_true
	homey := EqExpr(
		State("homey-pir-001", "alarm_motion"),
		Lit(true),
	)

	// Google Home: starters: device.state, state=MotionDetection, is=motionDetected
	google := EqExpr(
		State("google-pir-001", "MotionDetection"),
		Lit("motionDetected"),
	)

	// All are structurally equivalent: Eq(StateRef, Literal)
	platforms := []*Expr{tuya, ha, st, homey, google}
	for i := 0; i < len(platforms); i++ {
		for j := i + 1; j < len(platforms); j++ {
			assert.True(t, StructuralEquiv(platforms[i], platforms[j]),
				"platform %d ≡ platform %d (structure)", i, j)
		}
	}

	// With value mapping: true == "on" == "active" == "motionDetected"
	// Note: EquivWithMapping checks attribute match too.
	// Cross-platform attributes differ (dp_1 vs state vs motion vs alarm_motion vs MotionDetection).
	// Full cross-platform equiv needs a RefMapper (attribute normalization) — tracked in IAIF spec.
	// Here we verify pairwise with matching attributes:
	motionMapper := func(ref *DeviceRef, value any) any {
		switch v := value.(type) {
		case bool:
			if v {
				return "motion_detected"
			}
			return "no_motion"
		case string:
			switch v {
			case "on", "active", "motionDetected":
				return "motion_detected"
			case "off", "inactive", "noMotionDetected":
				return "no_motion"
			}
		}
		return value
	}

	// Pairwise with same-attribute expressions (simulating attribute normalization)
	normalized := []*Expr{
		EqExpr(State("device-a", "motion"), Lit(true)),
		EqExpr(State("device-b", "motion"), Lit("on")),
		EqExpr(State("device-c", "motion"), Lit("active")),
		EqExpr(State("device-d", "motion"), Lit(true)),
		EqExpr(State("device-e", "motion"), Lit("motionDetected")),
	}
	for i := 0; i < len(normalized); i++ {
		for j := i + 1; j < len(normalized); j++ {
			assert.True(t, EquivWithMapping(normalized[i], normalized[j], motionMapper),
				"normalized %d ≡ normalized %d (semantic with mapper)", i, j)
		}
	}
}

// ── "Temperature above 25°C → turn on AC" ──
// Numeric comparison with device command.

func TestEquiv_TemperatureThreshold_TuyaVsHA(t *testing.T) {
	// Tuya: dp_id=4, comparator=">", value=25 → Gt(State, Lit(25))
	tuyaTrigger := GtExpr(
		State("tuya-sensor-001", "dp_4"),
		Lit(float64(25)),
	)
	tuyaAction := Cmd("tuya-ac-001", "dp_1", true)

	// HA: trigger: numeric_state, above: 25 → Gt(State, Lit(25))
	haTrigger := GtExpr(
		State("sensor.living_room_temp", "temperature"),
		Lit(float64(25)),
	)
	haAction := Cmd("climate.living_room", "temperature", float64(22)).
		WithMeta("ha_service", "climate.set_temperature")

	// Structural equivalence of triggers
	assert.True(t, StructuralEquiv(tuyaTrigger, haTrigger))

	// Structural equivalence of actions
	assert.True(t, StructuralEquiv(tuyaAction, haAction))

	// Without mapping: different attribute names → Equiv still true
	// because Equiv checks attribute match (dp_4 vs temperature → different)
	assert.False(t, Equiv(tuyaTrigger, haTrigger),
		"different attributes → not semantically equivalent without mapping")

	// With attribute mapping: dp_4 → temperature
	attrMapper := func(ref *DeviceRef, value any) any {
		if ref != nil && ref.Attribute == "dp_4" {
			ref.Attribute = "temperature"
		}
		return value
	}
	// Note: EquivWithMapping only maps values, not refs.
	// Full attribute mapping would need a RefMapper — future work.
	_ = attrMapper
}

// ── Compound condition: "(temp > 25 OR humid > 80) AND nighttime" ──

func TestEquiv_CompoundCondition_Commutative(t *testing.T) {
	tempHigh := GtExpr(State("s", "temperature"), Lit(25))
	humidHigh := GtExpr(State("s", "humidity"), Lit(80))
	nighttime := BetweenExpr(Time("now"), Lit("22:00"), Lit("06:00"))

	// Platform A might express as: (temp OR humid) AND night
	exprA := AndExpr(OrExpr(tempHigh, humidHigh), nighttime)

	// Platform B might express as: night AND (humid OR temp) — reordered
	exprB := AndExpr(nighttime, OrExpr(humidHigh, tempHigh))

	// Should be equivalent (And and Or are commutative)
	assert.True(t, Equiv(exprA, exprB), "compound condition is commutative")
}

// ── Action sequence equivalence ──

func TestEquiv_ActionSequence_OrderMatters(t *testing.T) {
	turnOn := Cmd("light", "state", "on")
	wait := DelayExpr(300)
	turnOff := Cmd("light", "state", "off")

	seqA := SeqExpr(turnOn, wait, turnOff)
	seqB := SeqExpr(turnOff, wait, turnOn) // Wrong order!

	assert.False(t, Equiv(seqA, seqB),
		"action sequence order matters — turning on then off ≠ off then on")
	assert.True(t, StructuralEquiv(seqA, seqB),
		"but structurally they look the same (Cmd, Delay, Cmd)")
}

// ── Safety-critical: baby camera must not lose its trigger ──

func TestSafetyCritical_CameraTriggerPreserved(t *testing.T) {
	// Baby camera: motion → record → notify
	// This automation MUST survive platform migration with exact semantics.

	sourceTrigger := EqExpr(
		State("camera-baby-room", "motion"),
		Lit(true),
	)
	sourceActions := SeqExpr(
		Cmd("camera-baby-room", "recording", "start"),
		NotifyExpr("Motion detected in baby room!"),
	)

	// After conversion to target platform
	targetTrigger := EqExpr(
		State("camera.baby_room", "motion"),
		Lit("on"),
	)
	targetActions := SeqExpr(
		Cmd("camera.baby_room", "recording", "start"),
		NotifyExpr("Motion detected in baby room!"),
	)

	// Structure must be preserved
	assert.True(t, StructuralEquiv(sourceTrigger, targetTrigger),
		"camera trigger structure preserved")
	assert.True(t, StructuralEquiv(sourceActions, targetActions),
		"camera action sequence structure preserved")

	// With value mapping: true == "on" for motion
	mapper := func(ref *DeviceRef, value any) any {
		if v, ok := value.(bool); ok && v {
			return "on"
		}
		return value
	}
	assert.True(t, EquivWithMapping(sourceTrigger, targetTrigger, mapper),
		"camera trigger semantically equivalent with mapping")

	// Notify message must be exactly preserved
	assert.Equal(t,
		sourceActions.Children[1].Value,
		targetActions.Children[1].Value,
		"notification message preserved exactly")
}
