package rbac

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func newGinCtx(method, target string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(method, target, nil)
	c.Request = req
	return c, w
}

func TestExtractWorkspaceID_ParamId(t *testing.T) {
	wsID := uuid.New().String()
	c, _ := newGinCtx(http.MethodGet, "/workspaces/"+wsID)
	c.Params = gin.Params{{Key: "id", Value: wsID}}

	got, err := ExtractWorkspaceID(c)
	if err != nil {
		t.Fatalf("ExtractWorkspaceID error: %v", err)
	}
	if got != wsID {
		t.Fatalf("got %q want %q", got, wsID)
	}
}

func TestExtractWorkspaceID_Header(t *testing.T) {
	wsID := uuid.New().String()
	c, _ := newGinCtx(http.MethodGet, "/")
	c.Request.Header.Set("X-Workspace-Id", wsID)

	got, err := ExtractWorkspaceID(c)
	if err != nil {
		t.Fatalf("ExtractWorkspaceID error: %v", err)
	}
	if got != wsID {
		t.Fatalf("got %q want %q", got, wsID)
	}
}

func TestExtractWorkspaceID_Query(t *testing.T) {
	wsID := uuid.New().String()
	c, _ := newGinCtx(http.MethodGet, "/?workspace_id="+wsID)

	got, err := ExtractWorkspaceID(c)
	if err != nil {
		t.Fatalf("ExtractWorkspaceID error: %v", err)
	}
	if got != wsID {
		t.Fatalf("got %q want %q", got, wsID)
	}
}

func TestExtractWorkspaceID_Invalid(t *testing.T) {
	c, _ := newGinCtx(http.MethodGet, "/workspaces/not-a-uuid")
	c.Params = gin.Params{{Key: "id", Value: "not-a-uuid"}}

	_, err := ExtractWorkspaceID(c)
	if err == nil {
		t.Fatalf("expected error")
	}
}
