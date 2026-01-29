package web

import "github.com/gin-gonic/gin"

func (h *Handlers) render(c *gin.Context, page string, data gin.H) {
	if data == nil {
		data = gin.H{}
	}
	if _, ok := data["CSRF"]; !ok {
		data["CSRF"] = GenerateCSRF(h.CSRFSecret, h.CSRFTTL)
	}
	h.R.Render(c, page, data)
}
