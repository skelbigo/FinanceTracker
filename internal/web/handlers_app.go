package web

import "github.com/gin-gonic/gin"

func (h *Handlers) GetDashboard(c *gin.Context) {
	kicker := c.Query("flash")
	h.render(c, "app/dashboard.html", gin.H{
		"Title":     "Dashboard",
		"Kicker":    kicker,
		"BodyClass": "app-dark",
		"MainClass": "dash-main",
	})
}
