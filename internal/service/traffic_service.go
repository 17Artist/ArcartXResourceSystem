// SPDX-License-Identifier: GPL-3.0-or-later

package service

import (
	"arcartx-resource/internal/store"
	"log/slog"
	"sync/atomic"
)

type TrafficService struct {
	trafficStore *store.TrafficStore
	dailyLimit   atomic.Int64
}

func NewTrafficService(trafficStore *store.TrafficStore, dailyLimit int64) *TrafficService {
	s := &TrafficService{
		trafficStore: trafficStore,
	}
	s.dailyLimit.Store(dailyLimit)
	return s
}

func (s *TrafficService) Record(apiKeyID *int64, bytes int64) {
	if err := s.trafficStore.Record(apiKeyID, bytes); err != nil {
		slog.Error("记录流量失败", "error", err)
	}
}

func (s *TrafficService) CheckLimit(additionalBytes int64) bool {
	totalBytes, _, err := s.trafficStore.GetTodayTotal()
	if err != nil {
		slog.Error("获取今日流量失败", "error", err)
		return true
	}
	return totalBytes+additionalBytes <= s.dailyLimit.Load()
}

type TrafficStats struct {
	TodayBytes     int64   `json:"todayBytes"`
	TodayCount     int     `json:"todayCount"`
	TodayMB        float64 `json:"todayMB"`
	TodayGB        float64 `json:"todayGB"`
	DailyLimit     int64   `json:"dailyLimit"`
	DailyLimitGB   float64 `json:"dailyLimitGB"`
	RemainingBytes int64   `json:"remainingBytes"`
	RemainingGB    float64 `json:"remainingGB"`
	IsExceeded     bool    `json:"isLimitExceeded"`
}

func (s *TrafficService) GetStats() *TrafficStats {
	totalBytes, downloadCount, err := s.trafficStore.GetTodayTotal()
	if err != nil {
		slog.Error("获取流量统计失败", "error", err)
		totalBytes = 0
		downloadCount = 0
	}

	limit := s.dailyLimit.Load()
	remaining := limit - totalBytes
	if remaining < 0 {
		remaining = 0
	}

	return &TrafficStats{
		TodayBytes:     totalBytes,
		TodayCount:     downloadCount,
		TodayMB:        float64(totalBytes) / (1024 * 1024),
		TodayGB:        float64(totalBytes) / (1024 * 1024 * 1024),
		DailyLimit:     limit,
		DailyLimitGB:   float64(limit) / (1024 * 1024 * 1024),
		RemainingBytes: remaining,
		RemainingGB:    float64(remaining) / (1024 * 1024 * 1024),
		IsExceeded:     totalBytes >= limit,
	}
}

func (s *TrafficService) SetDailyLimit(limit int64) {
	s.dailyLimit.Store(limit)
}
