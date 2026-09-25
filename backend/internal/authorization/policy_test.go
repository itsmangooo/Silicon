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
		{"viewer", ProjectRead, true}, {"viewer", SecretWrite, false},
		{"unknown", ProjectRead, false},
	}
	for _, test := range tests {
		if got := Allowed(test.role, test.permission); got != test.allowed {
			t.Errorf("Allowed(%q,%q)=%v want %v", test.role, test.permission, got, test.allowed)
		}
	}
}
