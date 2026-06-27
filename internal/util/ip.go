// SPDX-License-Identifier: GPL-3.0-or-later

package util

import (
	"net"
	"strings"
)

// ParseClientIP 从请求头中提取客户端真实 IP
func ParseClientIP(xForwardedFor, xRealIP, remoteAddr string) string {
	if xff := strings.TrimSpace(xForwardedFor); xff != "" {
		// X-Forwarded-For 可能包含多个 IP，取第一个
		if idx := strings.IndexByte(xff, ','); idx > 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return xff
	}
	if xri := strings.TrimSpace(xRealIP); xri != "" {
		return xri
	}
	// 去掉端口号
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

// ValidateIP 验证 IP 地址格式（支持 IPv4、IPv6、CIDR）
func ValidateIP(ip string) bool {
	// 尝试 CIDR
	if strings.Contains(ip, "/") {
		_, _, err := net.ParseCIDR(ip)
		return err == nil
	}
	return net.ParseIP(ip) != nil
}

// IPMatchesWhitelist 检查 IP 是否在白名单中（支持 CIDR）
func IPMatchesWhitelist(clientIP string, whitelist []string) bool {
	if len(whitelist) == 0 {
		return true // 空白名单 = 允许所有
	}

	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}

	for _, entry := range whitelist {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		if strings.Contains(entry, "/") {
			// CIDR 匹配
			_, cidr, err := net.ParseCIDR(entry)
			if err == nil && cidr.Contains(ip) {
				return true
			}
		} else {
			// 精确匹配（使用 net.IP.Equal 处理 IPv4/IPv6 规范化）
			entryIP := net.ParseIP(entry)
			if entryIP != nil && entryIP.Equal(ip) {
				return true
			}
		}
	}

	return false
}
