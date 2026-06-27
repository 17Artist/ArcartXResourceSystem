// SPDX-License-Identifier: GPL-3.0-or-later

package service

import (
	"arcartx-resource/internal/store"
	"log/slog"
	"time"
)

type CleanupService struct {
	secStore     *store.SecurityStore
	trafficStore *store.TrafficStore
	linkStore    *store.SignedLinkStore
	stopCh       chan struct{}
}

func NewCleanupService(sec *store.SecurityStore, traffic *store.TrafficStore, link *store.SignedLinkStore) *CleanupService {
	s := &CleanupService{
		secStore:     sec,
		trafficStore: traffic,
		linkStore:    link,
		stopCh:       make(chan struct{}),
	}
	go s.loop()
	slog.Info("清理服务已启动，每小时执行一次")
	return s
}

func (s *CleanupService) loop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.run()
		case <-s.stopCh:
			return
		}
	}
}

func (s *CleanupService) run() {
	secDel, err := s.secStore.CleanOld(30)
	if err != nil {
		slog.Error("清理安全日志失败", "error", err)
	}
	trafficDel, err := s.trafficStore.CleanOld(90)
	if err != nil {
		slog.Error("清理流量记录失败", "error", err)
	}
	linkDel, err := s.linkStore.CleanExpired()
	if err != nil {
		slog.Error("清理签名链接失败", "error", err)
	}
	if secDel > 0 || trafficDel > 0 || linkDel > 0 {
		slog.Info("清理完成", "安全日志", secDel, "流量记录", trafficDel, "签名链接", linkDel)
	}
}

func (s *CleanupService) Shutdown() {
	close(s.stopCh)
	slog.Info("清理服务已关闭")
}
