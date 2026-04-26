package specvalidator

import "strings"

// ValidationError aggregates spec validation issues.
type ValidationError struct {
	Issues []string
}

func (e *ValidationError) Error() string {
	if len(e.Issues) == 0 {
		return "spec validation failed"
	}
	return "spec validation failed: " + strings.Join(e.Issues, "; ")
}

func (e *ValidationError) Add(issue string) {
	if strings.TrimSpace(issue) == "" {
		return
	}
	e.Issues = append(e.Issues, issue)
}

func (e *ValidationError) OrNil() error {
	if e == nil || len(e.Issues) == 0 {
		return nil
	}
	return e
}
