package workspaces

import (
	"context"
	"github.com/skelbigo/FinanceTracker/internal/identity"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeRoleRepo struct {
	role            string
	workspaceExists bool
}

func (r *fakeRoleRepo) GetUserRole(ctx context.Context, workspaceID, userID string) (string, error) {
	return r.role, nil
}

func (r *fakeRoleRepo) WorkspaceExists(ctx context.Context, workspaceID string) (bool, error) {
	return r.workspaceExists, nil
}

func testAuthMW() gin.HandlerFunc {
	return func(c *gin.Context) {
		if uid := c.GetHeader("X-User-ID"); uid != "" {
			c.Set(identity.CtxUserIDKey, uid)
		}
		c.Next()
	}
}

func TestRequireWorkspaceRole_NotMemberForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)

	wsID := uuid.New().String()
	userID := uuid.New().String()

	r := gin.New()
	r.Use(testAuthMW())

	repo := &fakeRoleRepo{role: "", workspaceExists: true}
	r.GET("/workspaces/:id/transactions", RequireWorkspaceRole(repo, RoleViewer), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/workspaces/"+wsID+"/transactions", nil)
	req.Header.Set("X-User-ID", userID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestRequireWorkspaceRole_ViewerCannotMutate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	wsID := uuid.New().String()
	userID := uuid.New().String()

	r := gin.New()
	r.Use(testAuthMW())

	repo := &fakeRoleRepo{role: string(RoleViewer), workspaceExists: true}
	r.POST("/workspaces/:id/transactions", RequireWorkspaceRole(repo, RoleMember), func(c *gin.Context) {
		c.Status(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodPost, "/workspaces/"+wsID+"/transactions", nil)
	req.Header.Set("X-User-ID", userID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestRequireWorkspaceRole_MemberCanMutate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	wsID := uuid.New().String()
	userID := uuid.New().String()

	r := gin.New()
	r.Use(testAuthMW())

	repo := &fakeRoleRepo{role: string(RoleMember), workspaceExists: true}
	r.POST("/workspaces/:id/budgets", RequireWorkspaceRole(repo, RoleMember), func(c *gin.Context) {
		c.Status(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodPost, "/workspaces/"+wsID+"/budgets", nil)
	req.Header.Set("X-User-ID", userID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d", http.StatusCreated, w.Code)
	}
}

func TestRequireWorkspaceRole_OwnerCanManageMembers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	wsID := uuid.New().String()
	userID := uuid.New().String()

	r := gin.New()
	r.Use(testAuthMW())

	repo := &fakeRoleRepo{role: string(RoleOwner), workspaceExists: true}
	r.PATCH("/workspaces/:id/members/:userId", RequireWorkspaceRole(repo, RoleOwner), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPatch, "/workspaces/"+wsID+"/members/"+uuid.New().String(), nil)
	req.Header.Set("X-User-ID", userID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected %d, got %d", http.StatusNoContent, w.Code)
	}
}
