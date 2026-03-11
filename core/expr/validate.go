package expr

import "fmt"

// ValidationError describes a structural problem in an expression tree.
type ValidationError struct {
	Path    string // e.g., "children[0].children[1]"
	Message string
}

func (e ValidationError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s: %s", e.Path, e.Message)
	}
	return e.Message
}

// Validate checks structural correctness of an expression tree.
// Returns all errors found (not just the first).
func Validate(e *Expr) []ValidationError {
	return validate(e, "")
}

func validate(e *Expr, path string) []ValidationError {
	if e == nil {
		return []ValidationError{{Path: path, Message: "nil expression"}}
	}

	var errs []ValidationError

	switch e.Op {
	// Comparison: exactly 2 children (left, right), except Between (3) and In (2+)
	case Eq, Ne, Gt, Ge, Lt, Le, Contains:
		if len(e.Children) != 2 {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("%s requires exactly 2 children, got %d", e.Op, len(e.Children)),
			})
		}
	case Between:
		if len(e.Children) != 3 {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("between requires exactly 3 children (val, low, high), got %d", len(e.Children)),
			})
		}
	case In:
		if len(e.Children) < 2 {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("in requires at least 2 children (val + 1 option), got %d", len(e.Children)),
			})
		}

	// Combinators: at least 1 child (Not=1, And/Or=2+)
	case Not:
		if len(e.Children) != 1 {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("not requires exactly 1 child, got %d", len(e.Children)),
			})
		}
	case And, Or:
		if len(e.Children) < 2 {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("%s requires at least 2 children, got %d", e.Op, len(e.Children)),
			})
		}
	case Seq, Parallel:
		if len(e.Children) < 1 {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("%s requires at least 1 child, got %d", e.Op, len(e.Children)),
			})
		}

	// Leaf nodes: no children, must have value or ref
	case Literal:
		if len(e.Children) != 0 {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: "literal must have no children",
			})
		}
		// Value can be nil (null literal), so no check needed
	case StateRef:
		if len(e.Children) != 0 {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: "state_ref must have no children",
			})
		}
		if e.Ref == nil {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: "state_ref requires a DeviceRef",
			})
		} else if e.Ref.DeviceID == "" {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: "state_ref DeviceRef must have a DeviceID",
			})
		}
	case TimeRef:
		if len(e.Children) != 0 {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: "time_ref must have no children",
			})
		}

	// Action nodes
	case Command:
		if e.Ref == nil {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: "command requires a DeviceRef",
			})
		}
	case Delay:
		if e.Value == nil {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: "delay requires a value (seconds)",
			})
		}
	case Notify:
		// Notify can have nil value (template-based)
	case Scene:
		if e.Value == nil {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: "scene requires a value (scene ID)",
			})
		}

	default:
		errs = append(errs, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("unknown operator: %s", e.Op),
		})
	}

	// Recurse into children
	for i, child := range e.Children {
		childPath := fmt.Sprintf("%s.children[%d]", path, i)
		if path == "" {
			childPath = fmt.Sprintf("children[%d]", i)
		}
		errs = append(errs, validate(child, childPath)...)
	}

	return errs
}

// IsValid returns true if the expression tree has no structural errors.
func IsValid(e *Expr) bool {
	return len(Validate(e)) == 0
}

// DeviceRefs collects all unique device IDs referenced in the expression tree.
func DeviceRefs(e *Expr) []string {
	seen := make(map[string]bool)
	collectRefs(e, seen)
	result := make([]string, 0, len(seen))
	for id := range seen {
		result = append(result, id)
	}
	return result
}

func collectRefs(e *Expr, seen map[string]bool) {
	if e == nil {
		return
	}
	if e.Ref != nil && e.Ref.DeviceID != "" {
		seen[e.Ref.DeviceID] = true
	}
	for _, child := range e.Children {
		collectRefs(child, seen)
	}
}

// Depth returns the maximum depth of the expression tree.
func Depth(e *Expr) int {
	if e == nil || len(e.Children) == 0 {
		return 1
	}
	maxChild := 0
	for _, child := range e.Children {
		d := Depth(child)
		if d > maxChild {
			maxChild = d
		}
	}
	return 1 + maxChild
}

// NodeCount returns the total number of nodes in the expression tree.
func NodeCount(e *Expr) int {
	if e == nil {
		return 0
	}
	count := 1
	for _, child := range e.Children {
		count += NodeCount(child)
	}
	return count
}
