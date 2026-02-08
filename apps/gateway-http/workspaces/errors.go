package workspaces

import "errors"

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrAlreadyMember    = errors.New("already a member")
	ErrLastOwner        = errors.New("cannot remove last owner")
	ErrCannotSelfDemote = errors.New("owner cannot slf demote")
	ErrInvalidRole      = errors.New("invalid role")
)
