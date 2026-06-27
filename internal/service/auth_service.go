// SPDX-License-Identifier: GPL-3.0-or-later

package service

import (
	"arcartx-resource/config"
	"arcartx-resource/internal/model"
	"arcartx-resource/internal/store"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	cfg         *config.Config
	adminStore  *store.AdminStore
	apiKeyStore *store.ApiKeyStore
	secStore    *store.SecurityStore
	keyCache    atomic.Value // 存储 map[string]*model.ApiKeyInfo
	initialized bool
}

func NewAuthService(cfg *config.Config, adminStore *store.AdminStore, apiKeyStore *store.ApiKeyStore, secStore *store.SecurityStore) *AuthService {
	s := &AuthService{
		cfg:         cfg,
		adminStore:  adminStore,
		apiKeyStore: apiKeyStore,
		secStore:    secStore,
	}
	s.keyCache.Store(make(map[string]*model.ApiKeyInfo))
	s.initDefaults()
	s.refreshKeyCache()
	return s
}

func (s *AuthService) initDefaults() {
	exists, _ := s.adminStore.Exists(s.cfg.Auth.DefaultAdminUser)
	if !exists {
		hash, err := bcrypt.GenerateFromPassword([]byte(s.cfg.Auth.DefaultAdminPass), bcrypt.DefaultCost)
		if err != nil {
			slog.Error("生成默认管理员密码哈希失败", "error", err)
			return
		}
		if err := s.adminStore.Create(s.cfg.Auth.DefaultAdminUser, string(hash)); err != nil {
			slog.Error("创建默认管理员失败", "error", err)
			return
		}
		slog.Info("创建默认管理员账户", "username", s.cfg.Auth.DefaultAdminUser)

		// 创建默认 API Key
		apiKey, err := s.CreateApiKey("default")
		if err != nil {
			slog.Error("创建默认API密钥失败", "error", err)
			return
		}
		// 只打印前缀，不打印完整密钥
		slog.Info("默认API密钥已创建（仅显示一次）", "prefix", apiKey[:8]+"...")
	}

	// 检查是否使用默认密码，持续警告
	if s.cfg.Auth.DefaultAdminPass == "admin123" {
		slog.Warn("⚠ 管理员使用默认密码，请尽快通过管理后台修改密码！")
	}
}

// Authenticate 验证管理员登录，返回用户对象以便判断 2FA 状态
func (s *AuthService) Authenticate(username, password string) (*model.AdminUser, error) {
	user, err := s.adminStore.GetByUsername(username)
	if err != nil {
		return nil, errors.New("用户名或密码错误")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("用户名或密码错误")
	}
	_ = s.adminStore.UpdateLastLogin(username)
	return user, nil
}

// ChangePassword 更改管理员密码
func (s *AuthService) ChangePassword(username, currentPassword, newPassword string) error {
	user, err := s.adminStore.GetByUsername(username)
	if err != nil {
		return errors.New("用户不存在")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return errors.New("当前密码错误")
	}
	if len(newPassword) < 8 {
		return errors.New("新密码长度不能少于8位")
	}
	// bcrypt 最大支持 72 字节输入
	if len(newPassword) > 72 {
		return errors.New("密码长度不能超过72个字符")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("密码处理失败")
	}
	return s.adminStore.UpdatePassword(username, string(hash))
}

// GenerateJWT 生成 JWT Token
func (s *AuthService) GenerateJWT(username string) (string, time.Time, error) {
	expiresAt := time.Now().Add(s.cfg.Auth.JWTExpiration)
	claims := jwt.MapClaims{
		"sub":  username,
		"type": "admin",
		"iss":  "arcartx-resource-system",
		"aud":  "arcartx-users",
		"iat":  time.Now().Unix(),
		"exp":  expiresAt.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(s.cfg.Auth.JWTSecret))
	if err != nil {
		return "", time.Time{}, err
	}
	return tokenStr, expiresAt, nil
}

// ValidateJWT 验证 JWT Token
func (s *AuthService) ValidateJWT(tokenStr string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.cfg.Auth.JWTSecret), nil
	},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuer("arcartx-resource-system"),
		jwt.WithAudience("arcartx-users"),
	)
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, errors.New("无效的令牌")
	}
	tokenType, _ := claims["type"].(string)
	if tokenType != "admin" {
		return nil, errors.New("无效的令牌类型")
	}
	return claims, nil
}

// --- API Key 管理 ---

func (s *AuthService) CreateApiKey(name string) (string, error) {
	raw, err := generateRandomKey(32)
	if err != nil {
		return "", err
	}
	hash := hashKey(raw)
	prefix := raw[:8]

	_, err = s.apiKeyStore.Create(name, hash, prefix)
	if err != nil {
		return "", err
	}
	s.refreshKeyCache()
	return raw, nil
}

func (s *AuthService) ResetApiKey(id int64) (string, error) {
	raw, err := generateRandomKey(32)
	if err != nil {
		return "", err
	}
	hash := hashKey(raw)
	prefix := raw[:8]

	if err := s.apiKeyStore.UpdateHash(id, hash, prefix); err != nil {
		return "", err
	}
	s.refreshKeyCache()
	return raw, nil
}

func (s *AuthService) DeleteApiKey(id int64) error {
	if err := s.apiKeyStore.Delete(id); err != nil {
		return err
	}
	s.refreshKeyCache()
	return nil
}

func (s *AuthService) ListApiKeys() ([]model.ApiKey, error) {
	return s.apiKeyStore.GetAll()
}

func (s *AuthService) GetApiKey(id int64) (*model.ApiKey, error) {
	return s.apiKeyStore.GetByID(id)
}

func (s *AuthService) UpdateApiKeyWhitelist(id int64, ips []string) error {
	var whitelist *string
	if len(ips) > 0 {
		joined := strings.Join(ips, ",")
		whitelist = &joined
	}
	if err := s.apiKeyStore.UpdateIPWhitelist(id, whitelist); err != nil {
		return err
	}
	s.refreshKeyCache()
	return nil
}

func (s *AuthService) GetApiKeyWhitelist(id int64) ([]string, error) {
	key, err := s.apiKeyStore.GetByID(id)
	if err != nil {
		return nil, err
	}
	if key.IPWhitelist == nil || *key.IPWhitelist == "" {
		return []string{}, nil
	}
	return strings.Split(*key.IPWhitelist, ","), nil
}

// ValidateApiKey 验证 API Key，返回 KeyInfo（从缓存）
func (s *AuthService) ValidateApiKey(rawKey string) *model.ApiKeyInfo {
	hash := hashKey(rawKey)
	cache := s.keyCache.Load().(map[string]*model.ApiKeyInfo)
	if info, ok := cache[hash]; ok && info.IsActive {
		// 异步更新 last_used_at
		go func() { _ = s.apiKeyStore.UpdateLastUsed(info.ID) }()
		return info
	}
	return nil
}

// refreshKeyCache 原子替换整个缓存 map，避免并发读写窗口
func (s *AuthService) refreshKeyCache() {
	keys, err := s.apiKeyStore.GetActive()
	if err != nil {
		slog.Error("刷新API密钥缓存失败", "error", err)
		return
	}

	newCache := make(map[string]*model.ApiKeyInfo, len(keys))
	for _, k := range keys {
		var ips []string
		if k.IPWhitelist != nil && *k.IPWhitelist != "" {
			for _, ip := range strings.Split(*k.IPWhitelist, ",") {
				ip = strings.TrimSpace(ip)
				if ip != "" {
					ips = append(ips, ip)
				}
			}
		}
		newCache[k.KeyHash] = &model.ApiKeyInfo{
			ID:          k.ID,
			KeyName:     k.KeyName,
			IPWhitelist: ips,
			IsActive:    k.IsActive,
		}
	}
	// 原子替换
	s.keyCache.Store(newCache)
	slog.Debug("API密钥缓存已刷新", "count", len(keys))
}

// LogSecurityEvent 记录安全事件
func (s *AuthService) LogSecurityEvent(eventType, ip string, userAgent *string, details *string) {
	if err := s.secStore.Log(eventType, ip, userAgent, nil, nil, details); err != nil {
		slog.Error("记录安全事件失败", "error", err)
	}
}

// --- TOTP 两步验证 ---

// GenerateChallengeToken 生成 2 分钟有效的临时 JWT
func (s *AuthService) GenerateChallengeToken(username string) (string, error) {
	claims := jwt.MapClaims{
		"sub":  username,
		"type": "2fa-challenge",
		"iss":  "arcartx-resource-system",
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(2 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.Auth.JWTSecret))
}

// ValidateChallengeToken 验证临时 JWT，返回 username
func (s *AuthService) ValidateChallengeToken(tokenStr string) (string, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(s.cfg.Auth.JWTSecret), nil
	},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuer("arcartx-resource-system"),
	)
	if err != nil {
		return "", errors.New("验证令牌无效或已过期")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", errors.New("验证令牌无效")
	}
	tokenType, _ := claims["type"].(string)
	if tokenType != "2fa-challenge" {
		return "", errors.New("令牌类型错误")
	}
	username, _ := claims["sub"].(string)
	if username == "" {
		return "", errors.New("令牌数据异常")
	}
	return username, nil
}

// TOTPSetupResult 包含 TOTP 设置信息
type TOTPSetupResult struct {
	Secret string `json:"secret"`
	URL    string `json:"url"`
}

// GenerateTOTPSetup 生成 TOTP 密钥和配置 URL（用于 QR 码）
func (s *AuthService) GenerateTOTPSetup(username string) (*TOTPSetupResult, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "ArcartX",
		AccountName: username,
		Period:      30,
		Digits:      6,
	})
	if err != nil {
		return nil, fmt.Errorf("生成TOTP密钥失败: %w", err)
	}
	return &TOTPSetupResult{
		Secret: key.Secret(),
		URL:    key.URL(),
	}, nil
}

// EnableTOTP 启用两步验证（需要验证一次 code 确认用户已正确配置）
func (s *AuthService) EnableTOTP(username, secret, code string) error {
	if !totp.Validate(code, secret) {
		return errors.New("验证码错误，请确认已正确配置验证器")
	}
	return s.adminStore.EnableTOTP(username, secret)
}

// DisableTOTP 关闭两步验证（需要密码确认）
func (s *AuthService) DisableTOTP(username, password string) error {
	user, err := s.adminStore.GetByUsername(username)
	if err != nil {
		return errors.New("用户不存在")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return errors.New("密码错误")
	}
	return s.adminStore.DisableTOTP(username)
}

// ValidateTOTP 验证 TOTP 码
func (s *AuthService) ValidateTOTP(username, code string) error {
	user, err := s.adminStore.GetByUsername(username)
	if err != nil {
		return errors.New("用户不存在")
	}
	if !user.TOTPEnabled || user.TOTPSecret == nil {
		return errors.New("未启用两步验证")
	}
	if !totp.Validate(code, *user.TOTPSecret) {
		return errors.New("验证码错误")
	}
	return nil
}

// GetTOTPStatus 获取用户 2FA 状态
func (s *AuthService) GetTOTPStatus(username string) (bool, error) {
	user, err := s.adminStore.GetByUsername(username)
	if err != nil {
		return false, err
	}
	return user.TOTPEnabled, nil
}

// IsDefaultPassword 检查用户是否仍在使用默认密码
func (s *AuthService) IsDefaultPassword(username string) bool {
	user, err := s.adminStore.GetByUsername(username)
	if err != nil {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(s.cfg.Auth.DefaultAdminPass)) == nil
}

// --- helpers ---

var keyGenMu sync.Mutex

func generateRandomKey(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}
