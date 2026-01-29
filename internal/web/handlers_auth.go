package web

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/skelbigo/FinanceTracker/internal/auth"
)

func (h *Handlers) GetLogin(c *gin.Context) {
	h.render(c, "auth/login.html", gin.H{
		"Title":      "Login",
		"Flash":      c.Query("flash"),
		"BodyClass":  "auth",
		"MainClass":  "auth-main",
		"HideHeader": true,
	})
}

func (h *Handlers) GetResetRequest(c *gin.Context) {
	h.render(c, "auth/reset_request.html", gin.H{
		"Title":      "Reset password",
		"Flash":      c.Query("flash"),
		"BodyClass":  "auth",
		"MainClass":  "auth-main",
		"HideHeader": true,
	})
}

func (h *Handlers) PostResetRequest(c *gin.Context) {
	email := c.PostForm("email")
	token, err := h.Auth.RequestPasswordReset(c.Request.Context(), email)
	if err != nil {
		h.render(c, "auth/reset_request.html", gin.H{
			"Title":      "Reset password",
			"Flash":      err.Error(),
			"BodyClass":  "auth",
			"MainClass":  "auth-main",
			"HideHeader": true,
		})
		return
	}

	if token != "" {
		c.Redirect(http.StatusSeeOther, "/reset/confirm?token="+url.QueryEscape(token)+"&flash=Dev+reset+link")
		return
	}

	c.Redirect(http.StatusSeeOther, "/login?flash=If+the+email+exists,+you%27ll+receive+a+reset+link")
}

func (h *Handlers) GetResetConfirm(c *gin.Context) {
	token := c.Query("token")
	h.render(c, "auth/reset_confirm.html", gin.H{
		"Title":      "Set new password",
		"Flash":      c.Query("flash"),
		"Token":      token,
		"BodyClass":  "auth",
		"MainClass":  "auth-main",
		"HideHeader": true,
	})
}

func (h *Handlers) PostResetConfirm(c *gin.Context) {
	token := c.PostForm("token")
	newPassword := c.PostForm("password")

	if err := h.Auth.ConfirmPasswordReset(c.Request.Context(), token, newPassword); err != nil {
		flash := "Could not reset password"
		if errors.Is(err, auth.ErrInvalidResetToken) {
			flash = "Reset link is invalid or expired"
		} else if err.Error() != "" {
			flash = err.Error()
		}
		h.render(c, "auth/reset_confirm.html", gin.H{
			"Title":      "Set new password",
			"Flash":      flash,
			"Token":      token,
			"BodyClass":  "auth",
			"MainClass":  "auth-main",
			"HideHeader": true,
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/login?flash=Password+updated")
}

func (h *Handlers) PostLogin(c *gin.Context) {
	req := auth.LoginRequest{
		Email:    c.PostForm("email"),
		Password: c.PostForm("password"),
	}

	resp, err := h.Auth.Login(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows), errors.Is(err, auth.ErrInvalidCredentials):
			h.render(c, "auth/login.html", gin.H{
				"Title":      "Login",
				"Flash":      "Invalid credentials",
				"BodyClass":  "auth",
				"MainClass":  "auth-main",
				"HideHeader": true,
			})
			return
		default:
			h.render(c, "auth/login.html", gin.H{
				"Title":      "Login",
				"Flash":      "Something went wrong. Please try again.",
				"BodyClass":  "auth",
				"MainClass":  "auth-main",
				"HideHeader": true,
			})
			return
		}
	}

	setAuthCookies(c, h.CookieCfg, resp.AccessToken, h.AccessTTL, resp.RefreshToken, h.RefreshTTL)
	c.Redirect(http.StatusSeeOther, "/app")
}

func (h *Handlers) GetRegister(c *gin.Context) {
	h.render(c, "auth/register.html", gin.H{
		"Title":      "Register",
		"Flash":      c.Query("flash"),
		"BodyClass":  "auth",
		"MainClass":  "auth-main",
		"HideHeader": true,
	})
}

func (h *Handlers) PostRegister(c *gin.Context) {
	req := auth.RegisterRequest{
		Name:     c.PostForm("name"),
		Email:    c.PostForm("email"),
		Password: c.PostForm("password"),
	}

	resp, err := h.Auth.Register(c.Request.Context(), req)
	if err != nil {
		flash := "Could not create account"
		if errors.Is(err, auth.ErrEmailTaken) {
			flash = "Email already exists"
		} else if err.Error() != "" {
			flash = err.Error()
		}

		h.render(c, "auth/register.html", gin.H{
			"Title":      "Register",
			"Flash":      flash,
			"BodyClass":  "auth",
			"MainClass":  "auth-main",
			"HideHeader": true,
		})
		return
	}

	setAuthCookies(c, h.CookieCfg, resp.AccessToken, h.AccessTTL, resp.RefreshToken, h.RefreshTTL)
	c.Redirect(http.StatusSeeOther, "/app")
}

func (h *Handlers) PostLogout(c *gin.Context) {
	refresh, err := c.Cookie(RefreshCookie)
	if err == nil && refresh != "" {
		_ = h.Auth.Logout(c.Request.Context(), auth.LogoutRequest{RefreshToken: refresh})
	}

	clearAuthCookies(c, h.CookieCfg)
	c.Redirect(http.StatusSeeOther, "/login?flash=Logged+out")
}
