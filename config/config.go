// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Auth       AuthConfig       `mapstructure:"auth"`
	Storage    StorageConfig    `mapstructure:"storage"`
	Security   SecurityConfig   `mapstructure:"security"`
	SignedLink SignedLinkConfig `mapstructure:"signed_link"`
	Traffic    TrafficConfig    `mapstructure:"traffic"`
	Database   DatabaseConfig   `mapstructure:"database"`
}

type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type AuthConfig struct {
	JWTSecret        string        `mapstructure:"jwt_secret"`
	JWTExpiration    time.Duration `mapstructure:"jwt_expiration"`
	DefaultAdminUser string        `mapstructure:"default_admin_user"`
	DefaultAdminPass string        `mapstructure:"default_admin_pass"`
}

type StorageConfig struct {
	UploadDir         string   `mapstructure:"upload_dir"`
	MaxFileSize       int64    `mapstructure:"max_file_size"`
	AllowedExtensions []string `mapstructure:"allowed_extensions"`
}

type SecurityConfig struct {
	CORSOrigins []string        `mapstructure:"cors_origins"`
	RateLimit   RateLimitConfig `mapstructure:"rate_limit"`
}

type RateLimitConfig struct {
	API      string `mapstructure:"api"`
	Download string `mapstructure:"download"`
	Login    string `mapstructure:"login"`
}

type SignedLinkConfig struct {
	MaxExpiration time.Duration `mapstructure:"max_expiration"`
	MaxDownloads  int           `mapstructure:"max_downloads"`
}

type TrafficConfig struct {
	DailyLimit int64 `mapstructure:"daily_limit"`
}

type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

func Load() (*Config, error) {
	v := viper.New()

	// 默认值
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "30s")
	v.SetDefault("server.write_timeout", "60s")

	v.SetDefault("auth.jwt_secret", "")
	v.SetDefault("auth.jwt_expiration", "1h")
	v.SetDefault("auth.default_admin_user", "admin")
	v.SetDefault("auth.default_admin_pass", "admin123")

	v.SetDefault("storage.upload_dir", "uploads")
	v.SetDefault("storage.max_file_size", 536870912) // 512MB
	v.SetDefault("storage.allowed_extensions", []string{"zip"})

	v.SetDefault("security.cors_origins", []string{"*"})
	v.SetDefault("security.rate_limit.api", "100/m")
	v.SetDefault("security.rate_limit.download", "30/m")
	v.SetDefault("security.rate_limit.login", "10/h")

	v.SetDefault("signed_link.max_expiration", "60m")
	v.SetDefault("signed_link.max_downloads", 10)

	v.SetDefault("traffic.daily_limit", 214748364800) // 200GB

	v.SetDefault("database.path", "data/database.db")

	// 读取配置文件
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

	// 环境变量
	v.SetEnvPrefix("ARCARTX")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("读取配置文件失败: %w", err)
		}
		// 配置文件不存在，使用默认值
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	// JWT Secret 自动生成
	if cfg.Auth.JWTSecret == "" {
		secret, err := loadOrGenerateJWTSecret(cfg.Database.Path)
		if err != nil {
			return nil, fmt.Errorf("生成JWT密钥失败: %w", err)
		}
		cfg.Auth.JWTSecret = secret
	}

	return &cfg, nil
}

func loadOrGenerateJWTSecret(dbPath string) (string, error) {
	dataDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return "", err
	}

	secretFile := filepath.Join(dataDir, ".jwt_secret")

	data, err := os.ReadFile(secretFile)
	if err == nil && len(data) > 0 {
		return strings.TrimSpace(string(data)), nil
	}

	// 生成新的 secret
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	secret := hex.EncodeToString(bytes)

	if err := os.WriteFile(secretFile, []byte(secret), 0600); err != nil {
		return "", err
	}

	return secret, nil
}

// ParseRateLimit 解析速率限制字符串，如 "100/m" → (100, time.Minute)
func ParseRateLimit(s string) (int, time.Duration, error) {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("无效的速率限制格式: %s", s)
	}

	var count int
	if _, err := fmt.Sscanf(parts[0], "%d", &count); err != nil {
		return 0, 0, fmt.Errorf("无效的速率限制数量: %s", parts[0])
	}

	var duration time.Duration
	switch parts[1] {
	case "s":
		duration = time.Second
	case "m":
		duration = time.Minute
	case "h":
		duration = time.Hour
	default:
		return 0, 0, fmt.Errorf("无效的速率限制时间单位: %s", parts[1])
	}

	return count, duration, nil
}
