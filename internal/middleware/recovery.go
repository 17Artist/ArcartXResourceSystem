// SPDX-License-Identifier: GPL-3.0-or-later

package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				slog.Error("panic recovered",
					"error", err,
					"path", c.Request.URL.Path,
					"method", c.Request.Method,
					"stack", stack,
				)
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error":   "服务器内部错误",
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}
