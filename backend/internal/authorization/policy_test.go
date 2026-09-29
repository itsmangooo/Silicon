package authorization

import "testing"

func TestRolePermissions(t *testing.T) {
	tests := []struct {
		role       string
		permission Permission
		allowed    bool
	}{
		{"owner", ProjectDelete, true}, {"admin", MemberManage, true},
		{"developer", DeploymentCreate, true}, {"developer", ProjectDelete, false},
		{"developer", EnvironmentUpdate, true}, {"developer", EnvironmentDelete, false},
		{"viewer", ProjectRead, true}, {"viewer", SecretWrite, false},
		{"owner", CloudDelete, true}, {"admin", CostRead, true},
		{"developer", CloudRead, true}, {"developer", CloudProvision, false},
		{"viewer", BudgetRead, true}, {"viewer", CostRead, false},
		{"unknown", ProjectRead, false},
	}
	for _, test := range tests {
		if got := Allowed(test.role, test.permission); got != test.allowed {
			t.Errorf("Allowed(%q,%q)=%v want %v", test.role, test.permission, got, test.allowed)
		}
	}
}
