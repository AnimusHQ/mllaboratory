package policy

import "testing"

func TestSpecValidate(t *testing.T) {
	spec := Spec{
		Schema: SpecSchemaV1,
		Rules: []Rule{
			{
				ID:     "allow-admin",
				Effect: EffectAllow,
				When: ConditionGroup{
					Any: []Condition{
						{Field: "user.roles", Op: "in", Values: []string{"admin"}},
					},
				},
			},
		},
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("Validate() err=%v", err)
	}

	invalid := spec
	invalid.Schema = "bad"
	if err := invalid.Validate(); err == nil {
		t.Fatalf("expected schema error")
	}
}

func TestEvaluateRuleOrder(t *testing.T) {
	spec := Spec{
		Schema:        SpecSchemaV1,
		DefaultEffect: EffectDeny,
		Rules: []Rule{
			{
				ID:     "high-gpu",
				Effect: EffectRequireApproval,
				When: ConditionGroup{
					All: []Condition{
						{Field: "resources.gpus", Op: "gte", Value: "8"},
					},
				},
			},
			{
				ID:     "allow-admin",
				Effect: EffectAllow,
				When: ConditionGroup{
					Any: []Condition{
						{Field: "user.roles", Op: "in", Values: []string{"admin"}},
					},
				},
			},
		},
	}

	decision, err := Evaluate(spec, Context{
		Actor: ActorContext{Subject: "alice", Roles: []string{"admin"}},
		Resources: map[string]any{
			"gpus": 16,
		},
	})
	if err != nil {
		t.Fatalf("Evaluate() err=%v", err)
	}
	if decision.Effect != EffectRequireApproval {
		t.Fatalf("Effect=%s, want %s", decision.Effect, EffectRequireApproval)
	}
	if decision.RuleID != "high-gpu" {
		t.Fatalf("RuleID=%s, want high-gpu", decision.RuleID)
	}
}

func TestEvaluateDefaultEffect(t *testing.T) {
	spec := Spec{
		Schema:        SpecSchemaV1,
		DefaultEffect: EffectAllow,
		Rules: []Rule{
			{
				ID:     "deny-prod",
				Effect: EffectDeny,
				When: ConditionGroup{
					All: []Condition{
						{Field: "git.ref", Op: "matches", Value: "^refs/heads/prod$"},
					},
				},
			},
		},
	}

	decision, err := Evaluate(spec, Context{
		Actor: ActorContext{Subject: "bob", Roles: []string{"viewer"}},
		Git:   GitContext{Ref: "refs/heads/dev"},
	})
	if err != nil {
		t.Fatalf("Evaluate() err=%v", err)
	}
	if decision.Effect != EffectAllow {
		t.Fatalf("Effect=%s, want %s", decision.Effect, EffectAllow)
	}
	if decision.RuleID != "" {
		t.Fatalf("RuleID=%s, want empty", decision.RuleID)
	}
}
