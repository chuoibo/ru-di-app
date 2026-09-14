package auth

import (
	"testing"

	"mobile/services/core/internal/domain/permissions"
)

// Roles guards the dev X-Actor-Roles header and permissions.Roles guards every
// decision; both are app.domain.permissions.ROLES. Two lists of one fact drift
// apart silently, so this test is where they must agree.
func TestRolesMatchThePermissionsTable(t *testing.T) {
	table := permissions.Roles()
	if len(table) != len(Roles) {
		t.Fatalf("auth.Roles has %d roles, permissions.Roles() has %d", len(Roles), len(table))
	}
	for _, role := range table {
		if !Roles[role] {
			t.Fatalf("role %q is in the permissions table but auth.Roles refuses it", role)
		}
	}
}
