// SPDX-License-Identifier: GPL-3.0-or-later

package middleware

import (
	"arcartx-resource/internal/util"

	"github.com/gin-gonic/gin"
)

// IPWhitelist 检查 API Key 的 IP 白名单
// 使用 Gin 的 ClientIP()（受 TrustedProxies 控制）
func IPWhitelist() gin.HandlerFunc {
	return func(c *gin.Context) {
		info := GetApiKeyInfo(c)
		if info == nil {
			c.Next()
			return
		}

		if len(info.IPWhitelist) == 0 {
			c.Next()
			return
		}

		clientIP := c.ClientIP()

		if !util.IPMatchesWhitelist(clientIP, info.IPWhitelist) {
			util.Forbidden(c, "IP地址不在白名单中")
			c.Abort()
			return
		}

		c.Next()
	}
}
