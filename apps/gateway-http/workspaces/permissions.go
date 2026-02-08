package workspaces

type Capability string

const (
	CapReadData        Capability = "read_data"
	CapWriteData       Capability = "write_data"
	CapManageWorkspace Capability = "manage_workspace"
)

var CapabilityMinRole = map[Capability]Role{
	CapReadData:        RoleViewer,
	CapWriteData:       RoleMember,
	CapManageWorkspace: RoleOwner,
}

func Can(role Role, cap Capability) bool {
	min, ok := CapabilityMinRole[cap]
	if !ok {
		return false
	}
	return RoleAtLeast(role, min)
}
