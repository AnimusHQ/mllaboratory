package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const SpecSchemaV1 = "animus.policy.v1"

const (
	EffectAllow           = "allow"
	EffectDeny            = "deny"
	EffectRequireApproval = "require_approval"
)

const (
	EffectDefault = ""
)

type Spec struct {
	Schema        string `json:"schema" yaml:"schema"`
	DefaultEffect string `json:"default_effect,omitempty" yaml:"default_effect,omitempty"`
	Rules         []Rule `json:"rules" yaml:"rules"`
}

type Rule struct {
	ID          string         `json:"id" yaml:"id"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Effect      string         `json:"effect" yaml:"effect"`
	When        ConditionGroup `json:"when" yaml:"when"`
}

type ConditionGroup struct {
	All []Condition `json:"all,omitempty" yaml:"all,omitempty"`
	Any []Condition `json:"any,omitempty" yaml:"any,omitempty"`
}

type Condition struct {
	Field  string   `json:"field" yaml:"field"`
	Op     string   `json:"op" yaml:"op"`
	Value  string   `json:"value,omitempty" yaml:"value,omitempty"`
	Values []string `json:"values,omitempty" yaml:"values,omitempty"`
}

func ParseSpec(input []byte) (Spec, error) {
	var spec Spec
	if err := yaml.Unmarshal(input, &spec); err != nil {
		return Spec{}, fmt.Errorf("decode spec: %w", err)
	}
	if err := spec.Validate(); err != nil {
		return Spec{}, err
	}
	return spec, nil
}

func (s Spec) MarshalJSON() ([]byte, error) {
	type alias Spec
	return json.Marshal(alias(s))
}

func (s Spec) Validate() error {
	if strings.TrimSpace(s.Schema) != SpecSchemaV1 {
		return fmt.Errorf("spec.schema must be %q", SpecSchemaV1)
	}
	if len(s.Rules) == 0 {
		return errors.New("spec.rules must be non-empty")
	}

	defaultEffect := strings.ToLower(strings.TrimSpace(s.DefaultEffect))
	if defaultEffect != "" && !isEffectAllowed(defaultEffect) {
		return fmt.Errorf("spec.default_effect unsupported: %q", s.DefaultEffect)
	}

	seen := make(map[string]struct{}, len(s.Rules))
	for i, rule := range s.Rules {
		ruleID := strings.TrimSpace(rule.ID)
		if ruleID == "" {
			return fmt.Errorf("spec.rules[%d].id is required", i)
		}
		if _, ok := seen[ruleID]; ok {
			return fmt.Errorf("spec.rules[%d].id must be unique (duplicate %q)", i, ruleID)
		}
		seen[ruleID] = struct{}{}

		effect := strings.ToLower(strings.TrimSpace(rule.Effect))
		if effect == "" {
			return fmt.Errorf("spec.rules[%d].effect is required", i)
		}
		if !isEffectAllowed(effect) {
			return fmt.Errorf("spec.rules[%d].effect unsupported: %q", i, rule.Effect)
		}

		if len(rule.When.All) == 0 && len(rule.When.Any) == 0 {
			return fmt.Errorf("spec.rules[%d].when must include all or any", i)
		}
		if err := validateConditions(rule.When.All, fmt.Sprintf("spec.rules[%d].when.all", i)); err != nil {
			return err
		}
		if err := validateConditions(rule.When.Any, fmt.Sprintf("spec.rules[%d].when.any", i)); err != nil {
			return err
		}
	}
	return nil
}

func validateConditions(conds []Condition, prefix string) error {
	for i, cond := range conds {
		field := strings.TrimSpace(cond.Field)
		if field == "" {
			return fmt.Errorf("%s[%d].field is required", prefix, i)
		}
		op := strings.ToLower(strings.TrimSpace(cond.Op))
		if op == "" {
			return fmt.Errorf("%s[%d].op is required", prefix, i)
		}
		if !isOpAllowed(op) {
			return fmt.Errorf("%s[%d].op unsupported: %q", prefix, i, cond.Op)
		}

		switch op {
		case "exists":
			continue
		case "in", "not_in":
			values := trimNonEmpty(cond.Values)
			if len(values) == 0 {
				return fmt.Errorf("%s[%d].values must be non-empty for %s", prefix, i, op)
			}
		default:
			if strings.TrimSpace(cond.Value) == "" {
				return fmt.Errorf("%s[%d].value is required for %s", prefix, i, op)
			}
		}
	}
	return nil
}

func isEffectAllowed(effect string) bool {
	switch strings.ToLower(strings.TrimSpace(effect)) {
	case EffectAllow, EffectDeny, EffectRequireApproval:
		return true
	default:
		return false
	}
}

func isOpAllowed(op string) bool {
	switch strings.ToLower(strings.TrimSpace(op)) {
	case "eq", "neq", "in", "not_in", "contains", "not_contains", "matches", "exists", "gt", "gte", "lt", "lte":
		return true
	default:
		return false
	}
}

func trimNonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, item := range values {
		v := strings.TrimSpace(item)
		if v == "" {
			continue
		}
		key := strings.ToLower(v)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, v)
	}
	return out
}
