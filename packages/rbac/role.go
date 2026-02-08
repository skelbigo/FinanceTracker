package rbac

type Role string

const (
	RoleViewer Role = "viewer"
	RoleMember Role = "member"
	RoleOwner  Role = "owner"
)

func Rank(r Role) int {
	switch r {
	case RoleViewer:
		return 1
	case RoleMember:
		return 2
	case RoleOwner:
		return 3
	default:
		return 0
	}
}

func AtLeast(actual, required Role) bool {
	return Rank(actual) >= Rank(required)
}

func IsValid(r Role) bool {
	switch r {
	case RoleViewer, RoleMember, RoleOwner:
		return true
	default:
		return false
	}
}
