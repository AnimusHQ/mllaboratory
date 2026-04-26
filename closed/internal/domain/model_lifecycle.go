package domain

import "fmt"

var modelTransitions = map[ModelStatus][]ModelStatus{
	ModelStatusDraft:      {ModelStatusValidated, ModelStatusDeprecated},
	ModelStatusValidated:  {ModelStatusApproved, ModelStatusDeprecated},
	ModelStatusApproved:   {ModelStatusDeprecated},
	ModelStatusDeprecated: {},
}

// CanTransition returns true when a transition is allowed.
func CanTransition(from, to ModelStatus) bool {
	allowed, ok := modelTransitions[from]
	if !ok {
		return false
	}
	for _, candidate := range allowed {
		if candidate == to {
			return true
		}
	}
	return false
}

// ValidateTransition ensures a model status transition is valid.
func ValidateTransition(from, to ModelStatus) error {
	if !from.Valid() || !to.Valid() {
		return fmt.Errorf("invalid model status transition")
	}
	if from == to {
		return nil
	}
	if !CanTransition(from, to) {
		return fmt.Errorf("model status transition %q -> %q not allowed", from, to)
	}
	return nil
}
