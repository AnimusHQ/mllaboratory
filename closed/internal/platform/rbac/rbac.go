package rbac

import (
	"context"
	"strings"

	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/repo"
)

type BindingStore interface {
	ListBySubjects(ctx context.Context, projectID string, subjects []repo.RoleBindingSubject) ([]repo.RoleBindingRecord, error)
}

type SubjectType string

const (
	SubjectTypeSubject SubjectType = "subject"
	SubjectTypeEmail   SubjectType = "email"
	SubjectTypeGroup   SubjectType = "group"
	SubjectTypeService SubjectType = "service"
)

var roleLevels = map[string]int{
	auth.RoleViewer: 1,
	auth.RoleEditor: 2,
	auth.RoleAdmin:  3,
}

func EffectiveRoleFromBindings(bindings []repo.RoleBindingRecord) string {
	maxLevel := 0
	chosen := ""
	for _, binding := range bindings {
		role := strings.ToLower(strings.TrimSpace(binding.Role))
		level := roleLevels[role]
		if level > maxLevel {
			maxLevel = level
			chosen = role
		}
	}
	return chosen
}

func EffectiveRoleFromIdentity(identity auth.Identity) string {
	maxLevel := 0
	chosen := ""
	for _, role := range identity.Roles {
		role = strings.ToLower(strings.TrimSpace(role))
		level := roleLevels[role]
		if level > maxLevel {
			maxLevel = level
			chosen = role
		}
	}
	return chosen
}

func HasAtLeast(role string, required string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	required = strings.ToLower(strings.TrimSpace(required))
	return roleLevels[role] >= roleLevels[required]
}

func SubjectsForIdentity(identity auth.Identity) []repo.RoleBindingSubject {
	subjects := make([]repo.RoleBindingSubject, 0)
	seen := make(map[string]struct{})
	add := func(kind SubjectType, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		key := string(kind) + ":" + strings.ToLower(value)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		subjects = append(subjects, repo.RoleBindingSubject{Type: string(kind), Value: value})
	}

	subject := strings.TrimSpace(identity.Subject)
	if subject != "" {
		add(SubjectTypeSubject, subject)
		if strings.HasPrefix(strings.ToLower(subject), "service:") {
			add(SubjectTypeService, subject)
		}
	}
	if strings.TrimSpace(identity.Email) != "" {
		add(SubjectTypeEmail, identity.Email)
	}
	for _, role := range identity.Roles {
		add(SubjectTypeGroup, role)
	}
	return subjects
}

func ResolveRole(ctx context.Context, store BindingStore, projectID string, identity auth.Identity, allowDirect bool) (string, []repo.RoleBindingRecord, error) {
	projectID = strings.TrimSpace(projectID)
	if store == nil || projectID == "" {
		if !allowDirect {
			return "", nil, nil
		}
		direct := EffectiveRoleFromIdentity(identity)
		return direct, nil, nil
	}

	bindings, err := store.ListBySubjects(ctx, projectID, SubjectsForIdentity(identity))
	if err != nil {
		return "", nil, err
	}
	bindingRole := EffectiveRoleFromBindings(bindings)
	if !allowDirect {
		return bindingRole, bindings, nil
	}
	direct := EffectiveRoleFromIdentity(identity)
	if HasAtLeast(direct, bindingRole) {
		return direct, bindings, nil
	}
	return bindingRole, bindings, nil
}

func IsRunToken(identity auth.Identity) bool {
	_, _, ok := auth.ParseRunTokenSubject(identity.Subject)
	return ok
}
