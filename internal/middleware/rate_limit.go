// SPDX-License-Identifier: GPL-3.0-or-later

package middleware

import (
	"arcartx-resource/internal/util"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type RateLimiter struct {
	visitors sync.Map
	limit    rate.Limit
	burst    int
	stopCh   chan struct{}
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen atomic.Int64 // Unix 秒，避免数据竞争
}

func NewRateLimiter(maxRequests int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		limit:  rate.Limit(float64(maxRequests) / window.Seconds()),
		burst:  maxRequests,
		stopCh: make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	v, ok := rl.visitors.Load(ip)
	if ok {
		vis := v.(*visitor)
		vis.lastSeen.Store(time.Now().Unix())
		return vis.limiter
	}
	vis := &visitor{limiter: rate.NewLimiter(rl.limit, rl.burst)}
	vis.lastSeen.Store(time.Now().Unix())
	rl.visitors.Store(ip, vis)
	return vis.limiter
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now().Unix()
			rl.visitors.Range(func(key, value interface{}) bool {
				vis := value.(*visitor)
				if now-vis.lastSeen.Load() > 1800 { // 30 分钟
					rl.visitors.Delete(key)
				}
				return true
			})
		case <-rl.stopCh:
			return
		}
	}
}

func (rl *RateLimiter) Shutdown() {
	close(rl.stopCh)
}

func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		limiter := rl.getLimiter(ip)
		if !limiter.Allow() {
			util.TooManyRequests(c, "请求过于频繁，请稍后再试")
			c.Abort()
			return
		}
		c.Next()
	}
}
