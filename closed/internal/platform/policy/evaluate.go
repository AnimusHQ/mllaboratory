package policy

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type Context struct {
	Actor      ActorContext           `json:"actor"`
	Dataset    DatasetContext         `json:"dataset"`
	Experiment ExperimentContext      `json:"experiment"`
	Git        GitContext             `json:"git"`
	Image      ImageContext           `json:"image"`
	Resources  map[string]any         `json:"resources,omitempty"`
	Labels     map[string]string      `json:"labels,omitempty"`
	Meta       map[string]interface{} `json:"meta,omitempty"`
}

type ActorContext struct {
	Subject string   `json:"subject"`
	Email   string   `json:"email,omitempty"`
	Roles   []string `json:"roles,omitempty"`
}

type DatasetContext struct {
	DatasetID string `json:"dataset_id,omitempty"`
	VersionID string `json:"version_id,omitempty"`
	SHA256    string `json:"sha256,omitempty"`
}

type ExperimentContext struct {
	ExperimentID string `json:"experiment_id,omitempty"`
	RunID        string `json:"run_id,omitempty"`
}

type GitContext struct {
	Repo   string `json:"repo,omitempty"`
	Commit string `json:"commit,omitempty"`
	Ref    string `json:"ref,omitempty"`
}

type ImageContext struct {
	Ref    string `json:"ref,omitempty"`
	Digest string `json:"digest,omitempty"`
}

type Decision struct {
	Effect      string `json:"effect"`
	RuleID      string `json:"rule_id,omitempty"`
	Description string `json:"description,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

func Evaluate(spec Spec, ctx Context) (Decision, error) {
	if err := spec.Validate(); err != nil {
		return Decision{}, err
	}
	for _, rule := range spec.Rules {
		if ruleMatches(rule, ctx) {
			return Decision{
				Effect:      normalizeEffect(rule.Effect),
				RuleID:      strings.TrimSpace(rule.ID),
				Description: strings.TrimSpace(rule.Description),
				Reason:      "rule_match",
			}, nil
		}
	}

	defaultEffect := normalizeEffect(spec.DefaultEffect)
	if defaultEffect == "" {
		defaultEffect = EffectDeny
	}
	return Decision{
		Effect: defaultEffect,
		Reason: "default",
	}, nil
}

func ruleMatches(rule Rule, ctx Context) bool {
	all := rule.When.All
	any := rule.When.Any

	if len(all) > 0 {
		for _, cond := range all {
			if !conditionMatches(cond, ctx) {
				return false
			}
		}
	}
	if len(any) > 0 {
		found := false
		for _, cond := range any {
			if conditionMatches(cond, ctx) {
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

func conditionMatches(cond Condition, ctx Context) bool {
	field := strings.TrimSpace(cond.Field)
	value, ok := ctx.Field(field)
	if !ok {
		return false
	}
	op := strings.ToLower(strings.TrimSpace(cond.Op))
	switch op {
	case "exists":
		return ok
	case "eq":
		return compareEqual(value, cond.Value)
	case "neq":
		return !compareEqual(value, cond.Value)
	case "in":
		return compareIn(value, cond.Values)
	case "not_in":
		return !compareIn(value, cond.Values)
	case "contains":
		return compareContains(value, cond.Value)
	case "not_contains":
		return !compareContains(value, cond.Value)
	case "matches":
		return compareRegex(value, cond.Value)
	case "gt", "gte", "lt", "lte":
		return compareNumber(value, cond.Value, op)
	default:
		return false
	}
}

func (c Context) Field(name string) (any, bool) {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return nil, false
	}
	switch key {
	case "user.subject", "actor.subject", "subject":
		return c.Actor.Subject, strings.TrimSpace(c.Actor.Subject) != ""
	case "user.email", "actor.email", "email":
		return c.Actor.Email, strings.TrimSpace(c.Actor.Email) != ""
	case "user.roles", "actor.roles", "roles", "role":
		if len(c.Actor.Roles) == 0 {
			return c.Actor.Roles, false
		}
		return c.Actor.Roles, true
	case "dataset.id", "dataset.dataset_id", "dataset_id":
		return c.Dataset.DatasetID, strings.TrimSpace(c.Dataset.DatasetID) != ""
	case "dataset.version_id", "dataset_version_id":
		return c.Dataset.VersionID, strings.TrimSpace(c.Dataset.VersionID) != ""
	case "dataset.sha256", "dataset.content_sha256":
		return c.Dataset.SHA256, strings.TrimSpace(c.Dataset.SHA256) != ""
	case "experiment.id", "experiment.experiment_id", "experiment_id":
		return c.Experiment.ExperimentID, strings.TrimSpace(c.Experiment.ExperimentID) != ""
	case "experiment.run_id", "run_id":
		return c.Experiment.RunID, strings.TrimSpace(c.Experiment.RunID) != ""
	case "git.repo", "git.repository":
		return c.Git.Repo, strings.TrimSpace(c.Git.Repo) != ""
	case "git.commit", "git.sha":
		return c.Git.Commit, strings.TrimSpace(c.Git.Commit) != ""
	case "git.ref":
		return c.Git.Ref, strings.TrimSpace(c.Git.Ref) != ""
	case "image.ref":
		return c.Image.Ref, strings.TrimSpace(c.Image.Ref) != ""
	case "image.digest", "image.sha256":
		return c.Image.Digest, strings.TrimSpace(c.Image.Digest) != ""
	}
	if strings.HasPrefix(key, "resources.") {
		value, ok := resolveMapPath(c.Resources, strings.TrimPrefix(key, "resources."))
		return value, ok
	}
	if strings.HasPrefix(key, "labels.") {
		value, ok := resolveStringMapPath(c.Labels, strings.TrimPrefix(key, "labels."))
		return value, ok
	}
	if strings.HasPrefix(key, "meta.") {
		value, ok := resolveMapPath(c.Meta, strings.TrimPrefix(key, "meta."))
		return value, ok
	}
	return nil, false
}

func resolveStringMapPath(values map[string]string, path string) (string, bool) {
	if len(values) == 0 {
		return "", false
	}
	key := strings.TrimSpace(path)
	if key == "" {
		return "", false
	}
	value, ok := values[key]
	return value, ok
}

func resolveMapPath(root map[string]any, path string) (any, bool) {
	if len(root) == 0 {
		return nil, false
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, false
	}
	parts := strings.Split(path, ".")
	var current any = root
	for _, part := range parts {
		key := strings.TrimSpace(part)
		if key == "" {
			return nil, false
		}
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[key]
			if !ok {
				return nil, false
			}
			current = next
		case []any:
			index, err := strconv.Atoi(key)
			if err != nil || index < 0 || index >= len(typed) {
				return nil, false
			}
			current = typed[index]
		default:
			return nil, false
		}
	}
	return current, true
}

func compareEqual(value any, target string) bool {
	target = normalizeString(target)
	switch typed := value.(type) {
	case string:
		return normalizeString(typed) == target
	case []string:
		for _, item := range typed {
			if normalizeString(item) == target {
				return true
			}
		}
		return false
	case []any:
		for _, item := range typed {
			if normalizeString(fmt.Sprint(item)) == target {
				return true
			}
		}
		return false
	default:
		return normalizeString(fmt.Sprint(value)) == target
	}
}

func compareIn(value any, targets []string) bool {
	if len(targets) == 0 {
		return false
	}
	normalized := make([]string, 0, len(targets))
	seen := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		val := normalizeString(t)
		if val == "" {
			continue
		}
		if _, ok := seen[val]; ok {
			continue
		}
		seen[val] = struct{}{}
		normalized = append(normalized, val)
	}
	if len(normalized) == 0 {
		return false
	}

	switch typed := value.(type) {
	case string:
		return sliceContains(normalized, normalizeString(typed))
	case []string:
		for _, item := range typed {
			if sliceContains(normalized, normalizeString(item)) {
				return true
			}
		}
		return false
	case []any:
		for _, item := range typed {
			if sliceContains(normalized, normalizeString(fmt.Sprint(item))) {
				return true
			}
		}
		return false
	default:
		return sliceContains(normalized, normalizeString(fmt.Sprint(value)))
	}
}

func compareContains(value any, target string) bool {
	target = normalizeString(target)
	if target == "" {
		return false
	}
	switch typed := value.(type) {
	case string:
		return strings.Contains(normalizeString(typed), target)
	case []string:
		for _, item := range typed {
			if normalizeString(item) == target {
				return true
			}
		}
		return false
	case []any:
		for _, item := range typed {
			if normalizeString(fmt.Sprint(item)) == target {
				return true
			}
		}
		return false
	default:
		return strings.Contains(normalizeString(fmt.Sprint(value)), target)
	}
}

func compareRegex(value any, pattern string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	switch typed := value.(type) {
	case string:
		return re.MatchString(typed)
	case []string:
		for _, item := range typed {
			if re.MatchString(item) {
				return true
			}
		}
		return false
	case []any:
		for _, item := range typed {
			if re.MatchString(fmt.Sprint(item)) {
				return true
			}
		}
		return false
	default:
		return re.MatchString(fmt.Sprint(value))
	}
}

func compareNumber(value any, target string, op string) bool {
	left, ok := toFloat64(value)
	if !ok {
		return false
	}
	right, ok := toFloat64(target)
	if !ok {
		return false
	}
	switch op {
	case "gt":
		return left > right
	case "gte":
		return left >= right
	case "lt":
		return left < right
	case "lte":
		return left <= right
	default:
		return false
	}
}

func toFloat64(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case string:
		return parseFloat(typed)
	default:
		return parseFloat(fmt.Sprint(typed))
	}
}

func parseFloat(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func sliceContains(values []string, target string) bool {
	for _, item := range values {
		if item == target {
			return true
		}
	}
	return false
}

func normalizeEffect(effect string) string {
	effect = strings.ToLower(strings.TrimSpace(effect))
	switch effect {
	case EffectAllow, EffectDeny, EffectRequireApproval:
		return effect
	default:
		return EffectDefault
	}
}

func normalizeString(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
