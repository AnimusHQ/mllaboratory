package auth

import "testing"

func TestResolveRolesFromClaimsIncludesGroupsAndMappedRoles(t *testing.T) {
	cfg := Config{
		RolesClaim:  "roles",
		GroupsClaim: "groups",
		GroupRoleMap: map[string]string{
			"ml-admin": "admin",
		},
	}
	claims := map[string]any{
		"roles":  []string{"viewer"},
		"groups": []string{"ml-admin", "ml-team"},
	}

	roles := resolveRolesFromClaims(cfg, claims)
	if !contains(roles, "viewer") {
		t.Fatalf("expected viewer role, got %v", roles)
	}
	if !contains(roles, "ml-admin") || !contains(roles, "ml-team") {
		t.Fatalf("expected group values in roles, got %v", roles)
	}
	if !contains(roles, "admin") {
		t.Fatalf("expected mapped admin role, got %v", roles)
	}
}

func TestResolveRolesFromClaimsFallsBackToRealmAccess(t *testing.T) {
	cfg := Config{
		RolesClaim:  "roles",
		GroupsClaim: "groups",
	}
	claims := map[string]any{
		"realm_access": map[string]any{
			"roles": []string{"admin", "viewer"},
		},
	}
	roles := resolveRolesFromClaims(cfg, claims)
	if !contains(roles, "admin") || !contains(roles, "viewer") {
		t.Fatalf("expected realm_access roles, got %v", roles)
	}
}
