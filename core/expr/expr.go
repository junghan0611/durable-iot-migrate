// Package expr defines a structured expression tree for IoT automation rules.
//
// This replaces the unstructured map[string]any Config with a typed AST
// that preserves semantic meaning across platform conversions.
//
// Design philosophy: Sussman's SDF (Software Design for Flexibility) —
// composable parts with standardized interfaces. Each Expr node is a
// combinator that can be freely composed.
//
// The key insight: flexibility that preserves structure IS durability.
// A well-typed expression tree is both flexible (any platform maps to it)
// and durable (meaning survives transformation).
//
//	Tuya:  dp_id=1, comparator="==", value=true
//	HA:    entity_id=switch.lamp, to="on"
//	Both → Expr{Op: Eq, Left: StateRef("switch.lamp"), Right: Literal("on")}
package expr

import "encoding/json"

// Op is an expression operator.
type Op string

const (
	// ── Comparison operators ──

	// Eq checks equality: state == value.
	Eq Op = "eq"
	// Ne checks inequality: state != value.
	Ne Op = "ne"
	// Gt checks greater than: state > value.
	Gt Op = "gt"
	// Ge checks greater than or equal: state >= value.
	Ge Op = "ge"
	// Lt checks less than: state < value.
	Lt Op = "lt"
	// Le checks less than or equal: state <= value.
	Le Op = "le"
	// Between checks range: low <= state <= high.
	Between Op = "between"
	// In checks membership: state in {a, b, c}.
	In Op = "in"
	// Contains checks substring: state contains "x".
	Contains Op = "contains"

	// ── Logical combinators ──

	// And requires all children to be true.
	And Op = "and"
	// Or requires any child to be true.
	Or Op = "or"
	// Not negates the child expression.
	Not Op = "not"

	// ── Sequence combinators ──

	// Seq executes children in order. [action1, delay, action2]
	Seq Op = "seq"
	// Parallel executes children concurrently.
	Parallel Op = "parallel"

	// ── Leaf nodes ──

	// Literal is a concrete value: "on", 25, true.
	Literal Op = "literal"
	// StateRef reads a device attribute: device.temperature.
	StateRef Op = "state_ref"
	// TimeRef references a time value: "22:00", sunrise+offset.
	TimeRef Op = "time_ref"

	// ── Action nodes ──

	// Command issues a device command: turn_on, set_temperature(22).
	Command Op = "command"
	// Delay waits for a duration.
	Delay Op = "delay"
	// Notify sends a notification.
	Notify Op = "notify"
	// Scene activates a scene or runs another automation.
	Scene Op = "scene"
)

// Expr is a node in the expression tree.
// It is the universal intermediate representation (IR) for IoT automation rules.
//
// Composition examples:
//
//	temperature > 25:
//	  {Op: Gt, Children: [{Op: StateRef, Ref: {ID: "sensor.temp"}}, {Op: Literal, Value: 25}]}
//
//	time between 22:00-06:00:
//	  {Op: Between, Children: [{Op: TimeRef, Value: "now"}, {Op: Literal, Value: "22:00"}, {Op: Literal, Value: "06:00"}]}
//
//	(temp > 25) AND (light == "off"):
//	  {Op: And, Children: [<temp>25>, <light==off>]}
type Expr struct {
	Op       Op             `json:"op"`
	Children []*Expr        `json:"children,omitempty"`
	Ref      *DeviceRef     `json:"ref,omitempty"`
	Value    any            `json:"value,omitempty"`
	Meta     map[string]any `json:"meta,omitempty"`
}

// DeviceRef identifies a device and its attribute across platforms.
//
// The key problem: Tuya uses "dp_id=1" on device "xxx",
// HA uses "entity_id=sensor.temperature", SmartThings uses
// "device.main.temperatureMeasurement.temperature".
// DeviceRef normalizes these into a single structure.
type DeviceRef struct {
	// DeviceID is the platform-native device identifier.
	DeviceID string `json:"device_id"`
	// Attribute is the property/capability being referenced.
	// HA: state attribute (e.g., "temperature", "state")
	// Tuya: dp_id (e.g., "dp_1", "dp_4")
	// SmartThings: capability.attribute (e.g., "switch.switch")
	// Google: trait.attribute (e.g., "OnOff.on")
	// Homey: capability (e.g., "onoff", "measure_temperature")
	Attribute string `json:"attribute,omitempty"`
	// Component for multi-component devices (ST: "main", HA: domain).
	Component string `json:"component,omitempty"`
}

// ── Constructors: SDF-style combinators ──

// Lit creates a literal value node.
func Lit(v any) *Expr { return &Expr{Op: Literal, Value: v} }

// State creates a device state reference.
func State(deviceID, attribute string) *Expr {
	return &Expr{Op: StateRef, Ref: &DeviceRef{DeviceID: deviceID, Attribute: attribute}}
}

// StateWithComponent creates a device state ref with a component.
func StateWithComponent(deviceID, component, attribute string) *Expr {
	return &Expr{Op: StateRef, Ref: &DeviceRef{
		DeviceID: deviceID, Attribute: attribute, Component: component,
	}}
}

// Time creates a time reference.
func Time(value string) *Expr { return &Expr{Op: TimeRef, Value: value} }

// ── Comparison combinators ──

// cmp creates a binary comparison expression.
func cmp(op Op, left, right *Expr) *Expr {
	return &Expr{Op: op, Children: []*Expr{left, right}}
}

// EqExpr creates an equality check: left == right.
func EqExpr(left, right *Expr) *Expr { return cmp(Eq, left, right) }

// NeExpr creates an inequality check: left != right.
func NeExpr(left, right *Expr) *Expr { return cmp(Ne, left, right) }

// GtExpr creates a greater-than check: left > right.
func GtExpr(left, right *Expr) *Expr { return cmp(Gt, left, right) }

// GeExpr creates a greater-or-equal check: left >= right.
func GeExpr(left, right *Expr) *Expr { return cmp(Ge, left, right) }

// LtExpr creates a less-than check: left < right.
func LtExpr(left, right *Expr) *Expr { return cmp(Lt, left, right) }

// LeExpr creates a less-or-equal check: left <= right.
func LeExpr(left, right *Expr) *Expr { return cmp(Le, left, right) }

// BetweenExpr creates a range check: low <= val <= high.
func BetweenExpr(val, low, high *Expr) *Expr {
	return &Expr{Op: Between, Children: []*Expr{val, low, high}}
}

// InExpr creates a membership check: val in set.
func InExpr(val *Expr, set ...*Expr) *Expr {
	children := make([]*Expr, 0, 1+len(set))
	children = append(children, val)
	children = append(children, set...)
	return &Expr{Op: In, Children: children}
}

// ── Logical combinators ──

// AndExpr combines conditions: all must be true.
func AndExpr(children ...*Expr) *Expr { return &Expr{Op: And, Children: children} }

// OrExpr combines conditions: any must be true.
func OrExpr(children ...*Expr) *Expr { return &Expr{Op: Or, Children: children} }

// NotExpr negates a condition.
func NotExpr(child *Expr) *Expr { return &Expr{Op: Not, Children: []*Expr{child}} }

// ── Sequence combinators ──

// SeqExpr creates a sequential execution: [a, then b, then c].
func SeqExpr(children ...*Expr) *Expr { return &Expr{Op: Seq, Children: children} }

// ParallelExpr creates parallel execution: [a and b and c simultaneously].
func ParallelExpr(children ...*Expr) *Expr { return &Expr{Op: Parallel, Children: children} }

// ── Action constructors ──

// Cmd creates a device command expression.
func Cmd(deviceID, attribute string, value any) *Expr {
	return &Expr{
		Op:    Command,
		Ref:   &DeviceRef{DeviceID: deviceID, Attribute: attribute},
		Value: value,
	}
}

// DelayExpr creates a delay action.
func DelayExpr(seconds float64) *Expr {
	return &Expr{Op: Delay, Value: seconds}
}

// NotifyExpr creates a notification action.
func NotifyExpr(message string) *Expr {
	return &Expr{Op: Notify, Value: message}
}

// SceneExpr creates a scene activation action.
func SceneExpr(sceneID string) *Expr {
	return &Expr{Op: Scene, Value: sceneID}
}

// ── Helpers ──

// WithMeta attaches platform-specific metadata to an expression.
// Returns the same Expr for chaining: Cmd(...).WithMeta("service", "light.turn_on")
func (e *Expr) WithMeta(key string, value any) *Expr {
	if e.Meta == nil {
		e.Meta = make(map[string]any)
	}
	e.Meta[key] = value
	return e
}

// IsLeaf returns true if this is a leaf node (no children).
func (e *Expr) IsLeaf() bool {
	return len(e.Children) == 0
}

// IsComparison returns true if this is a comparison operator.
func (e *Expr) IsComparison() bool {
	switch e.Op {
	case Eq, Ne, Gt, Ge, Lt, Le, Between, In, Contains:
		return true
	}
	return false
}

// IsCombinator returns true if this is a logical/sequence combinator.
func (e *Expr) IsCombinator() bool {
	switch e.Op {
	case And, Or, Not, Seq, Parallel:
		return true
	}
	return false
}

// IsAction returns true if this is an action node.
func (e *Expr) IsAction() bool {
	switch e.Op {
	case Command, Delay, Notify, Scene:
		return true
	}
	return false
}

// String returns a human-readable representation.
func (e *Expr) String() string {
	if e == nil {
		return "<nil>"
	}
	b, _ := json.Marshal(e)
	return string(b)
}
