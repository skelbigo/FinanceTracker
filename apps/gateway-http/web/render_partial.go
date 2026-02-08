package web

import (
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

func (r *Renderer) RenderPartial(c *gin.Context, tmplName string, data gin.H) {
	partials, _ := filepath.Glob(filepath.Join(r.templatesDir, "partials", "*.html"))

	tmpl, err := template.ParseFiles(partials...)
	if err != nil {
		c.String(http.StatusInternalServerError, "template parse error: %v", err)
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, tmplName, data); err != nil {
		c.String(http.StatusInternalServerError, "template exec error: %v", err)
		return
	}
}
