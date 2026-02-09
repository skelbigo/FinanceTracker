package web

import (
	"bytes"
	"html/template"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
)

var md = goldmark.New()

func markdown(input string) template.HTML {
	if input == "" {
		return ""
	}

	var buf bytes.Buffer
	if err := md.Convert([]byte(input), &buf); err != nil {
		return template.HTML(template.HTMLEscapeString(input))
	}

	p := bluemonday.UGCPolicy()
	safe := p.SanitizeBytes(buf.Bytes())

	return template.HTML(safe)
}

func RenderMarkdownSafe(input string) template.HTML { return markdown(input) }
