// SPDX-License-Identifier: GPL-3.0-or-later

package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

func CORS(origins []string) gin.HandlerFunc {
	allowAll := false
	for _, o := range origins {
		if o == "*" {
			allowAll = true
			break
		}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		if allowAll {
			// allowAll 时不设置 Credentials，防止 CSRF
			c.Header("Access-Control-Allow-Origin", "*")
		} else {
			// 按 Origin 协商响应，避免缓存把某来源的 ACAO 头返回给另一来源
			c.Header("Vary", "Origin")
			for _, o := range origins {
				if strings.EqualFold(o, origin) {
					c.Header("Access-Control-Allow-Origin", origin)
					c.Header("Access-Control-Allow-Credentials", "true")
					break
				}
			}
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
