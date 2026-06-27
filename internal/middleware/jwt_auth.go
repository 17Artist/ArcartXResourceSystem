// SPDX-License-Identifier: GPL-3.0-or-later

package middleware

import (
	"arcartx-resource/internal/service"
	"arcartx-resource/internal/util"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	ContextKeyUsername = "username"
	ContextKeyApiKey   = "api_key_info"
)

// JWTAuth JWT 认证中间件
func JWTAuth(auth *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			util.Unauthorized(c, "缺少认证令牌")
			c.Abort()
			return
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")
		claims, err := auth.ValidateJWT(tokenStr)
		if err != nil {
			util.Unauthorized(c, "无效的认证令牌")
			c.Abort()
			return
		}
		username, _ := claims["sub"].(string)
		c.Set(ContextKeyUsername, username)
		c.Next()
	}
}
