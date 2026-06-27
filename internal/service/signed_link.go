// SPDX-License-Identifier: GPL-3.0-or-later

package service

import (
	"arcartx-resource/internal/model"
	"arcartx-resource/internal/store"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"sync"
	"time"
)

type SignedLinkService struct {
	store  *store.SignedLinkStore
	cache  sync.Map   // token -> *model.SignedLink
	mu     sync.Mutex // 保护 ValidateAndConsume 的原子性
	stopCh chan struct{}
	maxExp time.Duration
	maxDL  int
}

func NewSignedLinkService(s *store.SignedLinkStore, maxExpiration time.Duration, maxDownloads int) *SignedLinkService {
	svc := &SignedLinkService{
		store:  s,
		stopCh: make(chan struct{}),
		maxExp: maxExpiration,
		maxDL:  maxDownloads,
	}
	svc.loadFromDB()
	go svc.cleanupLoop()
	slog.Info("签名链接服务初始化完成")
	return svc
}

func (s *SignedLinkService) loadFromDB() {
	links, err := s.store.LoadActive()
	if err != nil {
		slog.Error("加载签名链接失败", "error", err)
		return
	}
	for i := range links {
		cp := links[i] // 拷贝，避免循环变量引用
		s.cache.Store(cp.Token, &cp)
	}
	slog.Info("从数据库加载签名链接", "count", len(links))
}

func (s *SignedLinkService) Generate(fileName string, expirationMinutes, downloadLimit int, createdBy string) (*model.SignedLink, error) {
	if expirationMinutes <= 0 || time.Duration(expirationMinutes)*time.Minute > s.maxExp {
		return nil, errors.New("过期时间超出允许范围")
	}
	if downloadLimit <= 0 || downloadLimit > s.maxDL {
		return nil, errors.New("下载次数超出允许范围")
	}

	token, err := generateToken(32)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	link := &model.SignedLink{
		Token:              token,
		FileName:           fileName,
		ExpiresAt:          now.Add(time.Duration(expirationMinutes) * time.Minute),
		DownloadLimit:      downloadLimit,
		RemainingDownloads: downloadLimit,
		CreatedBy:          createdBy,
		CreatedAt:          now,
	}

	if err := s.store.Create(link); err != nil {
		return nil, err
	}
	s.cache.Store(token, link)

	slog.Info("生成签名链接", "file", fileName, "createdBy", createdBy, "expires", link.ExpiresAt)
	return link, nil
}

// Validate 验证签名链接但不消费（用于预检查）
func (s *SignedLinkService) Validate(token string) (*model.SignedLink, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.cache.Load(token)
	if !ok {
		link, err := s.store.GetByToken(token)
		if err != nil {
			return nil, errors.New("无效的下载令牌")
		}
		v = link
	}

	link := v.(*model.SignedLink)

	now := time.Now().UTC()
	if now.After(link.ExpiresAt) {
		s.cache.Delete(token)
		_ = s.store.Delete(token)
		return nil, errors.New("下载链接已过期")
	}

	if link.RemainingDownloads <= 0 {
		s.cache.Delete(token)
		_ = s.store.Delete(token)
		return nil, errors.New("下载次数已用完")
	}

	return link, nil
}

// Consume 消费一次下载次数（在文件和流量检查通过后调用）
func (s *SignedLinkService) Consume(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.cache.Load(token)
	if !ok {
		return
	}
	link := v.(*model.SignedLink)

	_ = s.store.DecrementDownloads(token)

	updated := *link
	updated.RemainingDownloads--

	if updated.RemainingDownloads <= 0 {
		s.cache.Delete(token)
		_ = s.store.Delete(token)
	} else {
		s.cache.Store(token, &updated)
	}
}

// ValidateAndConsume 验证并消费签名链接（线程安全）
// 使用 mutex 保护整个 check-and-decrement 流程，防止竞态条件
func (s *SignedLinkService) ValidateAndConsume(token string) (*model.SignedLink, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 先查内存
	v, ok := s.cache.Load(token)
	if !ok {
		// 查 DB
		link, err := s.store.GetByToken(token)
		if err != nil {
			return nil, errors.New("无效的下载令牌")
		}
		v = link
	}

	link := v.(*model.SignedLink)

	now := time.Now().UTC()
	if now.After(link.ExpiresAt) {
		s.cache.Delete(token)
		_ = s.store.Delete(token)
		return nil, errors.New("下载链接已过期")
	}

	if link.RemainingDownloads <= 0 {
		s.cache.Delete(token)
		_ = s.store.Delete(token)
		return nil, errors.New("下载次数已用完")
	}

	// 先在 DB 中原子扣减
	if err := s.store.DecrementDownloads(token); err != nil {
		return nil, errors.New("下载令牌处理失败")
	}

	// 更新内存缓存（创建副本避免共享指针问题）
	updated := *link
	updated.RemainingDownloads--

	if updated.RemainingDownloads <= 0 {
		s.cache.Delete(token)
		_ = s.store.Delete(token)
	} else {
		s.cache.Store(token, &updated)
	}

	slog.Info("签名下载验证成功", "file", link.FileName, "remaining", updated.RemainingDownloads)
	return &updated, nil
}

func (s *SignedLinkService) ActiveCount() int {
	count := 0
	s.cache.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

func (s *SignedLinkService) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.cleanup()
		case <-s.stopCh:
			return
		}
	}
}

func (s *SignedLinkService) cleanup() {
	now := time.Now().UTC()
	removed := 0
	s.cache.Range(func(key, value interface{}) bool {
		link := value.(*model.SignedLink)
		if now.After(link.ExpiresAt) || link.RemainingDownloads <= 0 {
			s.cache.Delete(key)
			removed++
		}
		return true
	})
	dbRemoved, _ := s.store.CleanExpired()
	if removed > 0 || dbRemoved > 0 {
		slog.Info("清理过期签名链接", "memory", removed, "db", dbRemoved)
	}
}

func (s *SignedLinkService) Shutdown() {
	close(s.stopCh)
	slog.Info("签名链接服务已关闭")
}

func (s *SignedLinkService) SetMaxExpiration(d time.Duration) { s.maxExp = d }
func (s *SignedLinkService) SetMaxDownloads(n int)            { s.maxDL = n }

func generateToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
