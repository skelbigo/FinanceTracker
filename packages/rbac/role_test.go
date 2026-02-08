package rbac

import "testing"

func TestAtLeast(t *testing.T) {
	cases := []struct {
		actual   Role
		required Role
		want     bool
	}{
		{RoleOwner, RoleOwner, true},
		{RoleOwner, RoleMember, true},
		{RoleOwner, RoleViewer, true},
		{RoleMember, RoleOwner, false},
		{RoleMember, RoleMember, true},
		{RoleMember, RoleViewer, true},
		{RoleViewer, RoleOwner, false},
		{RoleViewer, RoleMember, false},
		{RoleViewer, RoleViewer, true},
		{"", RoleViewer, false},
		{"admin", RoleViewer, false},
	}

	for _, c := range cases {
		if got := AtLeast(c.actual, c.required); got != c.want {
			t.Fatalf("AtLeast(%q,%q)=%v want %v", c.actual, c.required, got, c.want)
		}
	}
}
