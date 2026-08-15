package models

import "testing"

func TestSuperAdminHasAllPermissions(t *testing.T) {
	if !HasPermission(RoleSuperAdmin, PermUserDelete) {
		t.Fatal("super_admin should have user.delete")
	}
	if !HasPermission(RoleSuperAdmin, PermAgentConfig) {
		t.Fatal("super_admin should have agent.config")
	}
}

func TestModeratorRBAC(t *testing.T) {
	if !HasPermission(RoleModerator, PermUserBan) {
		t.Fatal("moderator should have user.ban")
	}
	if !HasPermission(RoleModerator, PermRoomDelete) {
		t.Fatal("moderator should have room.delete")
	}
	if HasPermission(RoleModerator, PermAgentConfig) {
		t.Fatal("moderator should not have agent.config")
	}
}

func TestOperatorRBAC(t *testing.T) {
	if !HasPermission(RoleOperator, PermSystemRead) {
		t.Fatal("operator should have system.read")
	}
	if HasPermission(RoleOperator, PermAgentConfig) {
		t.Fatal("operator should not have agent.config")
	}
	if HasPermission(RoleOperator, PermFileDelete) {
		t.Fatal("operator should not have file.delete")
	}
}
