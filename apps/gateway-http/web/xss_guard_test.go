package web

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"net/http/httptest"
)

func TestRenderer_EscapesUserContent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmp := t.TempDir()

	layout := `{{ define "layout" }}<!doctype html><html><body>{{ template "content" . }}</body></html>{{ end }}`
	page := `{{ define "content" }}<p id="x">{{ .Body }}</p>{{ end }}`

	if err := os.WriteFile(filepath.Join(tmp, "layout.html"), []byte(layout), 0o600); err != nil {
		t.Fatalf("write layout: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "page.html"), []byte(page), 0o600); err != nil {
		t.Fatalf("write page: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "partials"), 0o700); err != nil {
		t.Fatalf("mkdir partials: %v", err)
	}

	r := NewRenderer(tmp)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	payload := `<script>alert(1)</script>`
	r.Render(c, "page.html", gin.H{"Body": payload})

	out := w.Body.String()
	if strings.Contains(out, payload) {
		t.Fatalf("expected payload to be escaped, got raw output: %q", out)
	}
	if !strings.Contains(out, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatalf("expected escaped payload, got: %q", out)
	}
}

func TestWebUI_DoesNotUseUnsafeHTMLHelpers(t *testing.T) {
	bannedTemplate := []*regexp.Regexp{
		regexp.MustCompile(`\{\{\s*html\s+`),
		regexp.MustCompile(`\|\s*html\b`),
		regexp.MustCompile(`safeHTML|safeJS|safeURL|rawHTML`),
	}

	bannedCode := []*regexp.Regexp{
		regexp.MustCompile(`template\.(HTML|HTMLAttr|JS|JSStr|URL|CSS)\b`),
	}

	tmplRoot := filepath.Join("web", "templates")
	_ = filepath.WalkDir(tmplRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".html") {
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		s := string(b)
		for _, re := range bannedTemplate {
			if re.FindStringIndex(s) != nil {
				t.Fatalf("unsafe template helper found in %s (matched %q)", path, re.String())
			}
		}
		return nil
	})

	codeRoot := filepath.Join("apps", "gateway-http", "web")
	_ = filepath.WalkDir(codeRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		s := string(b)
		if filepath.Base(path) == "markdown.go" {
			re := regexp.MustCompile(`template\.(HTMLAttr|JS|JSStr|URL|CSS)\b`)
			if re.FindStringIndex(s) != nil {
				t.Fatalf("unsafe HTML type found in UI code %s (matched %q)", path, re.String())
			}
			return nil
		}
		for _, re := range bannedCode {
			if re.FindStringIndex(s) != nil {
				t.Fatalf("unsafe HTML type found in UI code %s (matched %q)", path, re.String())
			}
		}
		return nil
	})
}
