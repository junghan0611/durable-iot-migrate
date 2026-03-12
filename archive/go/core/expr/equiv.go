package expr

import "fmt"

// Equiv checks if two expression trees are semantically equivalent.
// This is the core of "durable semantics" — verifying that a converted
// automation still means the same thing.
//
// Two expressions are equivalent if:
//   - Same operator
//   - Same children (recursively), order-independent for And/Or
//   - Same DeviceRef (by attribute, device ID may differ across platforms)
//   - Same value for literals
//
// Example: Tuya "dp_1 == true" should be equivalent to HA "state == on"
// when there's a known mapping dp_1 → state, true → "on".
// Without mapping, we check structural equivalence only.
func Equiv(a, b *Expr) bool {
	return equiv(a, b, nil)
}

// EquivWithMapping checks equivalence with a value mapping function.
// The mapper translates platform-specific values to canonical form.
type ValueMapper func(ref *DeviceRef, value any) any

func EquivWithMapping(a, b *Expr, mapper ValueMapper) bool {
	return equiv(a, b, mapper)
}

func equiv(a, b *Expr, mapper ValueMapper) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Op != b.Op {
		return false
	}

	switch a.Op {
	case Literal:
		va, vb := a.Value, b.Value
		if mapper != nil {
			// Try to canonicalize values
			va = mapper(nil, va)
			vb = mapper(nil, vb)
		}
		return fmt.Sprintf("%v", va) == fmt.Sprintf("%v", vb)

	case StateRef:
		return refEquiv(a.Ref, b.Ref)

	case TimeRef:
		return fmt.Sprintf("%v", a.Value) == fmt.Sprintf("%v", b.Value)

	case And, Or:
		// Order-independent: every child in A has a match in B
		if len(a.Children) != len(b.Children) {
			return false
		}
		used := make([]bool, len(b.Children))
		for _, ac := range a.Children {
			found := false
			for j, bc := range b.Children {
				if !used[j] && equiv(ac, bc, mapper) {
					used[j] = true
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true

	case Not:
		if len(a.Children) != 1 || len(b.Children) != 1 {
			return false
		}
		return equiv(a.Children[0], b.Children[0], mapper)

	case Seq, Parallel:
		// Order-dependent for Seq, order-independent for Parallel
		if len(a.Children) != len(b.Children) {
			return false
		}
		if a.Op == Parallel {
			// Same as And/Or: order-independent
			used := make([]bool, len(b.Children))
			for _, ac := range a.Children {
				found := false
				for j, bc := range b.Children {
					if !used[j] && equiv(ac, bc, mapper) {
						used[j] = true
						found = true
						break
					}
				}
				if !found {
					return false
				}
			}
			return true
		}
		// Seq: order matters
		for i := range a.Children {
			if !equiv(a.Children[i], b.Children[i], mapper) {
				return false
			}
		}
		return true

	default:
		// Comparison, Command, etc: check children in order + ref + value
		if len(a.Children) != len(b.Children) {
			return false
		}
		for i := range a.Children {
			if !equiv(a.Children[i], b.Children[i], mapper) {
				return false
			}
		}
		if !refEquiv(a.Ref, b.Ref) {
			return false
		}
		if a.Value != nil || b.Value != nil {
			va, vb := a.Value, b.Value
			if mapper != nil {
				va = mapper(a.Ref, va)
				vb = mapper(b.Ref, vb)
			}
			return fmt.Sprintf("%v", va) == fmt.Sprintf("%v", vb)
		}
		return true
	}
}

func refEquiv(a, b *DeviceRef) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// Attribute must match; DeviceID may differ across platforms
	return a.Attribute == b.Attribute
}

// StructuralEquiv checks if two trees have the same shape (operators and arity)
// without checking values or refs. Useful for "same pattern, different devices".
func StructuralEquiv(a, b *Expr) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Op != b.Op {
		return false
	}
	if len(a.Children) != len(b.Children) {
		return false
	}
	for i := range a.Children {
		if !StructuralEquiv(a.Children[i], b.Children[i]) {
			return false
		}
	}
	return true
}
