package handlers

import (
	"context"
	stderrors "errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

const statusClientClosedRequest = 499

func handleRequestContextError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}

	switch {
	case stderrors.Is(err, context.Canceled), stderrors.Is(c.Request.Context().Err(), context.Canceled):
		c.AbortWithStatus(statusClientClosedRequest)
		return true
	case stderrors.Is(err, context.DeadlineExceeded), stderrors.Is(c.Request.Context().Err(), context.DeadlineExceeded):
		c.AbortWithStatus(http.StatusGatewayTimeout)
		return true
	default:
		return false
	}
}
