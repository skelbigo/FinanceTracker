package web

import (
	"strings"
	"testing"
)

func TestRenderMarkdownSafe_StripsScriptsAndJsUrls(t *testing.T) {
	input := "Hello <script>alert(1)</script> **bold** [x](javascript:alert(1))"
	out := string(RenderMarkdownSafe(input))

	if contains(out, "<script") {
		t.Fatalf("expected <script> tag to be stripped, got: %q", out)
	}
	if contains(out, "javascript:") {
		t.Fatalf("expected javascript: URL to be stripped, got: %q", out)
	}
	if !contains(out, "<strong>") {
		t.Fatalf("expected markdown formatting to remain, got: %q", out)
	}
}

func TestRenderMarkdownSafe_RemovesDangerousAttributes(t *testing.T) {
	input := "<img src=x onerror=alert(1)>"
	out := string(RenderMarkdownSafe(input))
	if contains(out, "onerror") || contains(out, "<img") {
		t.Fatalf("expected dangerous tag/attrs to be stripped, got: %q", out)
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
