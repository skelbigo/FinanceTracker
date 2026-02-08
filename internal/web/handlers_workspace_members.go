package web

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/skelbigo/FinanceTracker/internal/identity"
	"github.com/skelbigo/FinanceTracker/internal/workspaces"
)

func (h *Handlers) GetWorkspaceMembersPage(c *gin.Context) {
	if h.Workspaces == nil {
		c.String(http.StatusInternalServerError, "workspaces service is not configured")
		return
	}

	v, ok := c.Get(identity.CtxUserIDKey)
	userID, _ := v.(string)
	if !ok || userID == "" {
		c.Redirect(http.StatusSeeOther, "/login?flash=Please+login")
		return
	}

	wsID := strings.TrimSpace(c.Param("id"))
	if wsID == "" {
		c.Redirect(http.StatusSeeOther, "/app/workspaces?flash="+url.QueryEscape("Workspace not found"))
		return
	}

	w, myRole, ok := h.getWorkspaceForUserOrRedirect(c, wsID, userID)
	if !ok {
		return
	}

	members, err := h.Workspaces.ListMembers(c.Request.Context(), wsID)
	if err != nil {
		c.String(http.StatusInternalServerError, "could not list members")
		return
	}

	ownersCount := 0
	for _, m := range members {
		if m.Role == workspaces.RoleOwner {
			ownersCount++
		}
	}

	headerItems, _ := h.Workspaces.ListMyWorkspaces(c.Request.Context(), userID)
	current, _ := c.Cookie(CurrentWorkspaceCookie)

	h.render(c, "app/workspace_members.html", gin.H{
		"Title":            "Workspace",
		"BodyClass":        "app-dark app-solid",
		"MainClass":        "ws-main",
		"Flash":            c.Query("flash"),
		"Workspace":        w,
		"MyRole":           string(myRole),
		"Members":          members,
		"OwnersCount":      ownersCount,
		"CurrentUserID":    userID,
		"HeaderWorkspaces": headerItems,
		"CurrentID":        current,
		"ReturnTo":         c.Request.URL.Path,
	})
}

func (h *Handlers) PostAddWorkspaceMember(c *gin.Context) {
	if h.Workspaces == nil {
		c.String(http.StatusInternalServerError, "workspaces service is not configured")
		return
	}

	v, ok := c.Get(identity.CtxUserIDKey)
	userID, _ := v.(string)
	if !ok || userID == "" {
		c.Redirect(http.StatusSeeOther, "/login?flash=Please+login")
		return
	}

	wsID := strings.TrimSpace(c.Param("id"))
	w, myRole, ok := h.getWorkspaceForUserOrRedirect(c, wsID, userID)
	if !ok {
		return
	}

	if myRole != workspaces.RoleOwner {
		c.Redirect(http.StatusSeeOther, "/app/workspaces/"+url.PathEscape(w.ID)+"?flash="+url.QueryEscape("Only owners can manage members"))
		return
	}

	identifier := strings.TrimSpace(c.PostForm("identifier"))
	email := strings.TrimSpace(c.PostForm("email"))
	userIDInput := strings.TrimSpace(c.PostForm("user_id"))
	if identifier == "" {
		if email != "" {
			identifier = email
		} else {
			identifier = userIDInput
		}
	}
	role := workspaces.Role(strings.TrimSpace(c.PostForm("role")))
	if role == "" {
		role = workspaces.RoleMember
	}

	if identifier == "" {
		c.Redirect(http.StatusSeeOther, "/app/workspaces/"+url.PathEscape(w.ID)+"?flash="+url.QueryEscape("Email or User ID is required"))
		return
	}

	var err error
	if strings.Contains(identifier, "@") {
		err = h.Workspaces.AddMemberByEmail(c.Request.Context(), wsID, identifier, role)
	} else {
		if _, parseErr := uuid.Parse(identifier); parseErr != nil {
			c.Redirect(http.StatusSeeOther, "/app/workspaces/"+url.PathEscape(w.ID)+"?flash="+url.QueryEscape("Invalid user id"))
			return
		}
		err = h.Workspaces.AddMemberByUserID(c.Request.Context(), wsID, identifier, role)
	}
	if err != nil {
		msg := "Could not add member"
		switch {
		case errors.Is(err, workspaces.ErrUserNotFound):
			msg = "User not found"
		case errors.Is(err, workspaces.ErrAlreadyMember):
			msg = "User is already a member"
		case errors.Is(err, workspaces.ErrInvalidRole):
			msg = "Invalid role"
		default:
			if err.Error() != "" {
				msg = err.Error()
			}
		}
		c.Redirect(http.StatusSeeOther, "/app/workspaces/"+url.PathEscape(w.ID)+"?flash="+url.QueryEscape(msg))
		return
	}

	c.Redirect(http.StatusSeeOther, "/app/workspaces/"+url.PathEscape(w.ID)+"?flash="+url.QueryEscape("Member added"))
}

func (h *Handlers) PostUpdateWorkspaceMemberRole(c *gin.Context) {
	if h.Workspaces == nil {
		c.String(http.StatusInternalServerError, "workspaces service is not configured")
		return
	}

	v, ok := c.Get(identity.CtxUserIDKey)
	userID, _ := v.(string)
	if !ok || userID == "" {
		c.Redirect(http.StatusSeeOther, "/login?flash=Please+login")
		return
	}

	wsID := strings.TrimSpace(c.Param("id"))
	w, myRole, ok := h.getWorkspaceForUserOrRedirect(c, wsID, userID)
	if !ok {
		return
	}

	if myRole != workspaces.RoleOwner {
		c.Redirect(http.StatusSeeOther, "/app/workspaces/"+url.PathEscape(w.ID)+"?flash="+url.QueryEscape("Only owners can manage members"))
		return
	}

	targetUserID := strings.TrimSpace(c.Param("userId"))
	newRole := workspaces.Role(strings.TrimSpace(c.PostForm("role")))
	if targetUserID == "" || newRole == "" {
		c.Redirect(http.StatusSeeOther, "/app/workspaces/"+url.PathEscape(w.ID)+"?flash="+url.QueryEscape("Invalid request"))
		return
	}

	err := h.Workspaces.UpdateMemberRole(c.Request.Context(), wsID, userID, targetUserID, newRole)
	if err != nil {
		msg := "Could not update role"
		switch {
		case errors.Is(err, workspaces.ErrCannotSelfDemote):
			msg = "You cannot change your own role as owner"
		case errors.Is(err, workspaces.ErrLastOwner):
			msg = "Cannot change role: last owner"
		case errors.Is(err, workspaces.ErrInvalidRole):
			msg = "Invalid role"
		default:
			if err.Error() != "" {
				msg = err.Error()
			}
		}
		c.Redirect(http.StatusSeeOther, "/app/workspaces/"+url.PathEscape(w.ID)+"?flash="+url.QueryEscape(msg))
		return
	}

	c.Redirect(http.StatusSeeOther, "/app/workspaces/"+url.PathEscape(w.ID)+"?flash="+url.QueryEscape("Role updated"))
}

func (h *Handlers) PostRemoveWorkspaceMember(c *gin.Context) {
	if h.Workspaces == nil {
		c.String(http.StatusInternalServerError, "workspaces service is not configured")
		return
	}

	v, ok := c.Get(identity.CtxUserIDKey)
	userID, _ := v.(string)
	if !ok || userID == "" {
		c.Redirect(http.StatusSeeOther, "/login?flash=Please+login")
		return
	}

	wsID := strings.TrimSpace(c.Param("id"))
	w, myRole, ok := h.getWorkspaceForUserOrRedirect(c, wsID, userID)
	if !ok {
		return
	}

	if myRole != workspaces.RoleOwner {
		c.Redirect(http.StatusSeeOther, "/app/workspaces/"+url.PathEscape(w.ID)+"?flash="+url.QueryEscape("Only owners can manage members"))
		return
	}

	targetUserID := strings.TrimSpace(c.Param("userId"))
	if targetUserID == "" {
		c.Redirect(http.StatusSeeOther, "/app/workspaces/"+url.PathEscape(w.ID)+"?flash="+url.QueryEscape("Invalid request"))
		return
	}

	err := h.Workspaces.RemoveMember(c.Request.Context(), wsID, userID, targetUserID)
	if err != nil {
		msg := "Could not remove member"
		switch {
		case errors.Is(err, workspaces.ErrLastOwner):
			msg = "Cannot remove last owner"
		default:
			if err.Error() != "" {
				msg = err.Error()
			}
		}
		c.Redirect(http.StatusSeeOther, "/app/workspaces/"+url.PathEscape(w.ID)+"?flash="+url.QueryEscape(msg))
		return
	}

	c.Redirect(http.StatusSeeOther, "/app/workspaces/"+url.PathEscape(w.ID)+"?flash="+url.QueryEscape("Member removed"))
}

func (h *Handlers) getWorkspaceForUserOrRedirect(c *gin.Context, workspaceID, userID string) (workspaces.Workspace, workspaces.Role, bool) {
	w, role, err := h.Workspaces.GetWorkspace(c.Request.Context(), workspaceID, userID)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/app/workspaces?flash="+url.QueryEscape("You are not a member of that workspace"))
		return workspaces.Workspace{}, "", false
	}
	return w, role, true
}
