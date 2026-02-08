package web

import (
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

type Renderer struct {
	templatesDir string
}

func NewRenderer(templatesDir string) *Renderer {
	return &Renderer{templatesDir: templatesDir}
}

func (r *Renderer) Render(c *gin.Context, page string, data gin.H) {
	layout := filepath.Join(r.templatesDir, "layout.html")

	partials, _ := filepath.Glob(filepath.Join(r.templatesDir, "partials", "*.html"))

	pagePath := filepath.Join(r.templatesDir, page)

	files := []string{layout, pagePath}
	files = append(files, partials...)

	tmpl, err := template.ParseFiles(files...)
	if err != nil {
		c.String(http.StatusInternalServerError, "template parse error: %v", err)
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "layout", data); err != nil {
		c.String(http.StatusInternalServerError, "template exec error: %v", err)
		return
	}
}
