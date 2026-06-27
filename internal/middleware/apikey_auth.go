// SPDX-License-Identifier: GPL-3.0-or-later

package middleware

import (
	"arcartx-resource/internal/model"
	"arcartx-resource/internal/service"
	"arcartx-resource/internal/util"
	"strings"

	"github.com/gin-gonic/gin"
)

// ApiKeyAuth API Key 认证中间件
func ApiKeyAuth(auth *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			util.Unauthorized(c, "缺少认证令牌")
			c.Abort()
			return
		}
		rawKey := strings.TrimPrefix(header, "Bearer ")

		info := auth.ValidateApiKey(rawKey)
		if info != nil {
			c.Set(ContextKeyApiKey, info)
			c.Next()
			return
		}

		// 尝试 JWT
		claims, err := auth.ValidateJWT(rawKey)
		if err == nil {
			username, _ := claims["sub"].(string)
			c.Set(ContextKeyUsername, username)
			c.Next()
			return
		}

		util.Unauthorized(c, "无效的认证令牌")
		c.Abort()
	}
}

// GetApiKeyInfo 从 context 获取 API Key 信息
func GetApiKeyInfo(c *gin.Context) *model.ApiKeyInfo {
	v, exists := c.Get(ContextKeyApiKey)
	if !exists {
		return nil
	}
	info, ok := v.(*model.ApiKeyInfo)
	if !ok {
		return nil
	}
	return info
}

// GetUsername 从 context 获取用户名
func GetUsername(c *gin.Context) string {
	v, _ := c.Get(ContextKeyUsername)
	s, _ := v.(string)
	return s
}
