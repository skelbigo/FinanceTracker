package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handlers) GetLogin(c *gin.Context) {
	h.R.Render(c, "auth/login.html", gin.H{
		"Title":      "Login",
		"Flash":      c.Query("flash"),
		"BodyClass":  "auth",
		"MainClass":  "auth-main",
		"HideHeader": true,
	})
}

func (h *Handlers) PostLogin(c *gin.Context) {
	email := c.PostForm("email")
	_ = email

	c.Redirect(http.StatusSeeOther, "/app?flash=Logged+in")
}

func (h *Handlers) GetRegister(c *gin.Context) {
	h.R.Render(c, "auth/register.html", gin.H{
		"Title":      "Register",
		"Flash":      c.Query("flash"),
		"BodyClass":  "auth",
		"MainClass":  "auth-main",
		"HideHeader": true,
	})
}

func (h *Handlers) PostRegister(c *gin.Context) {
	c.Redirect(http.StatusSeeOther, "/login?flash=Account+created")
}

func (h *Handlers) PostLogout(c *gin.Context) {
	c.Redirect(http.StatusSeeOther, "/login?flash=Logged+out")
}
