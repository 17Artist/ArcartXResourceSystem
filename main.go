// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"arcartx-resource/config"
	"arcartx-resource/internal/handler"
	"arcartx-resource/internal/middleware"
	"arcartx-resource/internal/service"
	"arcartx-resource/internal/store"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("加载配置失败", "error", err)
		os.Exit(1)
	}

	db, err := store.New(cfg.Database.Path)
	if err != nil {
		slog.Error("初始化数据库失败", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// 初始化 Store
	adminStore := store.NewAdminStore(db)
	apiKeyStore := store.NewApiKeyStore(db)
	fileStore := store.NewFileStore(db)
	settingStore := store.NewSettingStore(db)
	securityStore := store.NewSecurityStore(db)
	trafficStore := store.NewTrafficStore(db)
	signedLinkStore := store.NewSignedLinkStore(db)

	initDefaultSettings(settingStore)

	// 初始化 Service
	authSvc := service.NewAuthService(cfg, adminStore, apiKeyStore, securityStore)
	fileSvc := service.NewFileService(cfg, fileStore)
	signedLinkSvc := service.NewSignedLinkService(signedLinkStore, cfg.SignedLink.MaxExpiration, cfg.SignedLink.MaxDownloads)
	captchaSvc := service.NewCaptchaService(1000)
	trafficSvc := service.NewTrafficService(trafficStore, cfg.Traffic.DailyLimit)
	cleanupSvc := service.NewCleanupService(securityStore, trafficStore, signedLinkStore)

	// 初始化 Handler
	authHandler := handler.NewAuthHandler(authSvc, captchaSvc)
	fileHandler := handler.NewFileHandler(fileSvc, signedLinkSvc, trafficSvc, cfg.Storage.MaxFileSize)
	downloadHandler := handler.NewDownloadHandler(signedLinkSvc, fileSvc, trafficSvc)
	apiKeyHandler := handler.NewApiKeyHandler(authSvc)
	configHandler := handler.NewConfigHandler(settingStore, trafficSvc, signedLinkSvc, fileHandler)

	// 速率限制器
	apiRL, downloadRL, loginRL := createRateLimiters(cfg)

	// 设置 Gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// 仅信任 loopback 代理，防止 X-Forwarded-For 伪造
	r.SetTrustedProxies([]string{"127.0.0.1", "::1"})

	r.Use(middleware.Recovery())
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.Logger())
	r.Use(middleware.CORS(cfg.Security.CORSOrigins))

	// 静态资源
	r.StaticFS("/static", http.FS(getStaticFS()))

	// /admin 快捷入口 → 直接返回 admin.html（支持 hash 路由）
	r.GET("/admin", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/static/admin.html")
	})

	// 健康检查（不泄露内部状态）
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"status":    "healthy",
				"timestamp": time.Now().UnixMilli(),
			},
			"message": "运行正常",
		})
	})

	// 认证路由
	auth := r.Group("/api/auth")
	{
		auth.GET("/captcha", authHandler.GetCaptcha)
		auth.POST("/login", loginRL.Middleware(), authHandler.Login)
		auth.POST("/validate", loginRL.Middleware(), authHandler.Validate)
		auth.POST("/change-password", middleware.JWTAuth(authSvc), authHandler.ChangePassword)

		// 2FA 公开端点（使用 challengeToken 认证）
		auth.POST("/2fa/verify", loginRL.Middleware(), authHandler.Verify2FA)

		// 2FA 管理端点（需要 JWT）
		twoFA := auth.Group("/2fa", middleware.JWTAuth(authSvc))
		{
			twoFA.GET("/status", authHandler.Status2FA)
			twoFA.POST("/setup", authHandler.Setup2FA)
			twoFA.POST("/enable", authHandler.Enable2FA)
			twoFA.POST("/disable", authHandler.Disable2FA)
		}
	}

	// 文件管理路由 (JWT)
	filesJWT := r.Group("/api/files", middleware.JWTAuth(authSvc))
	{
		filesJWT.GET("/list", fileHandler.List)
		filesJWT.POST("/upload", fileHandler.Upload)
		filesJWT.DELETE("/:fileName", fileHandler.Delete)
	}

	// 文件路由 (JWT 或 API Key)
	filesAuth := r.Group("/api/files", apiRL.Middleware(), middleware.ApiKeyAuth(authSvc), middleware.IPWhitelist())
	{
		filesAuth.GET("/crc64-list", fileHandler.CRC64List)
		filesAuth.POST("/generate-signed-link", fileHandler.GenerateSignedLink)
	}

	// 管理路由 (JWT)
	admin := r.Group("/api/admin", middleware.JWTAuth(authSvc))
	{
		// 流量统计（独立路径，避免与 /:id 冲突）
		admin.GET("/traffic-stats", func(c *gin.Context) {
			stats := trafficSvc.GetStats()
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data":    stats,
			})
		})

		keys := admin.Group("/keys")
		{
			keys.GET("/list", apiKeyHandler.List)
			keys.POST("/create", apiKeyHandler.Create)
		}
		// 带 :id 的路由单独注册，避免与固定路径冲突
		keyByID := admin.Group("/key")
		{
			keyByID.POST("/reset/:id", apiKeyHandler.Reset)
			keyByID.DELETE("/:id", apiKeyHandler.Delete)
			keyByID.GET("/:id/whitelist", apiKeyHandler.GetWhitelist)
			keyByID.POST("/:id/whitelist", apiKeyHandler.UpdateWhitelist)
		}

		cfgGroup := admin.Group("/config")
		{
			cfgGroup.GET("/list", configHandler.List)
			cfgGroup.POST("/update", configHandler.Update)
		}
	}

	// 下载路由 (公开，签名验证)
	r.GET("/api/download/signed/:token", downloadRL.Middleware(), downloadHandler.SignedDownload)

	// 启动服务器
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		slog.Info("ArcartX资源管理启动成功", "port", cfg.Server.Port)
		slog.Info("管理后台", "url", fmt.Sprintf("http://localhost:%d/admin", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("服务器启动失败", "error", err)
			os.Exit(1)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("正在关闭...")

	// 先停止接受新请求
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("服务器关闭失败", "error", err)
	}

	// 再关闭后台服务
	signedLinkSvc.Shutdown()
	cleanupSvc.Shutdown()
	apiRL.Shutdown()
	downloadRL.Shutdown()
	loginRL.Shutdown()

	slog.Info("关闭完成")
}

func createRateLimiters(cfg *config.Config) (*middleware.RateLimiter, *middleware.RateLimiter, *middleware.RateLimiter) {
	apiCount, apiWindow, err := config.ParseRateLimit(cfg.Security.RateLimit.API)
	if err != nil {
		apiCount, apiWindow = 100, time.Minute
	}
	dlCount, dlWindow, err := config.ParseRateLimit(cfg.Security.RateLimit.Download)
	if err != nil {
		dlCount, dlWindow = 30, time.Minute
	}
	loginCount, loginWindow, err := config.ParseRateLimit(cfg.Security.RateLimit.Login)
	if err != nil {
		loginCount, loginWindow = 10, time.Hour
	}

	return middleware.NewRateLimiter(apiCount, apiWindow),
		middleware.NewRateLimiter(dlCount, dlWindow),
		middleware.NewRateLimiter(loginCount, loginWindow)
}

func initDefaultSettings(s *store.SettingStore) {
	defaults := map[string]struct{ Value, Desc string }{
		"daily_traffic_limit":       {"214748364800", "全局每日流量限制（字节）"},
		"download_rate_limit":       {"30", "单IP下载频率限制（次/分钟）"},
		"login_rate_limit":          {"10", "单IP登录频率限制（次/小时）"},
		"max_file_size":             {"536870912", "单文件大小上限（字节）"},
		"signed_link_max_minutes":   {"60", "签名链接时效上限（分钟）"},
		"signed_link_max_downloads": {"10", "签名链接次数上限"},
	}
	if err := s.InitDefaults(defaults); err != nil {
		slog.Error("初始化默认配置失败", "error", err)
	}
}
