package httpx

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/errorsx"
)

func WriteError(c *gin.Context, err error) {
	if err == nil {
		Internal(c)
		return
	}

	var app *errorsx.Error
	if errors.As(err, &app) {
		switch app.Kind {
		case errorsx.KindNotFound:
			NotFound(c, app.Message)
			return
		case errorsx.KindForbidden:
			Error(c, http.StatusForbidden, app.Message, nil)
			return
		case errorsx.KindValidation:
			Unprocessable(c, app.Message, app.Details)
			return
		default:
			Internal(c)
			return
		}
	}

	Internal(c)
}
