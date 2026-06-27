// SPDX-License-Identifier: GPL-3.0-or-later

package handler

import (
	"arcartx-resource/internal/middleware"
	"arcartx-resource/internal/service"
	"arcartx-resource/internal/util"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	auth    *service.AuthService
	captcha *service.CaptchaService
}

func NewAuthHandler(auth *service.AuthService, captcha *service.CaptchaService) *AuthHandler {
	return &AuthHandler{auth: auth, captcha: captcha}
}

func (h *AuthHandler) GetCaptcha(c *gin.Context) {
	id, b64, err := h.captcha.Generate()
	if err != nil {
		slog.Error("生成验证码失败", "error", err)
		util.InternalError(c, "生成验证码失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"captchaId":    id,
		"captchaImage": b64,
		"message":      "验证码生成成功",
	})
}

type loginRequest struct {
	Username  string `json:"username" binding:"required"`
	Password  string `json:"password" binding:"required"`
	Captcha   string `json:"captcha" binding:"required"`
	CaptchaID string `json:"captchaId" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "请求格式错误")
		return
	}

	clientIP := c.ClientIP()
	ua := c.GetHeader("User-Agent")

	if !h.captcha.Validate(req.CaptchaID, req.Captcha) {
		details := "验证码验证失败: " + req.Username
		h.auth.LogSecurityEvent("LOGIN_CAPTCHA_FAILED", clientIP, &ua, &details)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "验证码错误"})
		return
	}

	user, err := h.auth.Authenticate(req.Username, req.Password)
	if err != nil {
		details := "登录失败: " + req.Username
		h.auth.LogSecurityEvent("LOGIN_FAILED", clientIP, &ua, &details)
		slog.Warn("登录失败", "username", req.Username, "ip", clientIP)
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "用户名或密码错误"})
		return
	}

	// 检查是否启用了两步验证
	if user.TOTPEnabled {
		challengeToken, err := h.auth.GenerateChallengeToken(req.Username)
		if err != nil {
			slog.Error("生成2FA挑战令牌失败", "error", err)
			util.InternalError(c, "生成验证令牌失败")
			return
		}
		details := "需要两步验证: " + req.Username
		h.auth.LogSecurityEvent("LOGIN_2FA_REQUIRED", clientIP, &ua, &details)
		c.JSON(http.StatusOK, gin.H{
			"success":        true,
			"requires2FA":    true,
			"challengeToken": challengeToken,
			"message":        "请输入两步验证码",
		})
		return
	}

	// 未启用 2FA，直接签发 JWT
	token, expiresAt, err := h.auth.GenerateJWT(req.Username)
	if err != nil {
		slog.Error("生成JWT失败", "error", err)
		util.InternalError(c, "生成令牌失败")
		return
	}

	details := "管理员登录成功: " + req.Username
	h.auth.LogSecurityEvent("LOGIN_SUCCESS", clientIP, &ua, &details)
	slog.Info("管理员登录成功", "username", req.Username, "ip", clientIP)

	c.JSON(http.StatusOK, gin.H{
		"success":            true,
		"token":              token,
		"message":            "登录成功",
		"expiresAt":          expiresAt,
		"mustChangePassword": h.auth.IsDefaultPassword(req.Username),
	})
}

// --- 2FA 验证 ---

type verify2FARequest struct {
	ChallengeToken string `json:"challengeToken" binding:"required"`
	Code           string `json:"code" binding:"required"`
}

func (h *AuthHandler) Verify2FA(c *gin.Context) {
	var req verify2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "请求格式错误")
		return
	}

	clientIP := c.ClientIP()
	ua := c.GetHeader("User-Agent")

	username, err := h.auth.ValidateChallengeToken(req.ChallengeToken)
	if err != nil {
		details := "2FA挑战令牌无效"
		h.auth.LogSecurityEvent("2FA_CHALLENGE_INVALID", clientIP, &ua, &details)
		util.Unauthorized(c, "验证令牌无效或已过期，请重新登录")
		return
	}

	if err := h.auth.ValidateTOTP(username, req.Code); err != nil {
		details := "2FA验证失败: " + username
		h.auth.LogSecurityEvent("2FA_VERIFY_FAILED", clientIP, &ua, &details)
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "验证码错误"})
		return
	}

	token, expiresAt, err := h.auth.GenerateJWT(username)
	if err != nil {
		slog.Error("生成JWT失败", "error", err)
		util.InternalError(c, "生成令牌失败")
		return
	}

	details := "管理员登录成功(2FA): " + username
	h.auth.LogSecurityEvent("LOGIN_SUCCESS_2FA", clientIP, &ua, &details)
	slog.Info("管理员登录成功(2FA)", "username", username, "ip", clientIP)

	c.JSON(http.StatusOK, gin.H{
		"success":            true,
		"token":              token,
		"message":            "登录成功",
		"expiresAt":          expiresAt,
		"mustChangePassword": h.auth.IsDefaultPassword(username),
	})
}

// --- 2FA 管理 ---

func (h *AuthHandler) Status2FA(c *gin.Context) {
	username := middleware.GetUsername(c)
	enabled, err := h.auth.GetTOTPStatus(username)
	if err != nil {
		util.InternalError(c, "获取状态失败")
		return
	}
	util.OK(c, gin.H{"enabled": enabled}, "")
}

func (h *AuthHandler) Setup2FA(c *gin.Context) {
	username := middleware.GetUsername(c)
	result, err := h.auth.GenerateTOTPSetup(username)
	if err != nil {
		slog.Error("生成TOTP设置失败", "error", err)
		util.InternalError(c, "生成两步验证配置失败")
		return
	}
	util.OK(c, gin.H{
		"secret": result.Secret,
		"url":    result.URL,
	}, "")
}

type enable2FARequest struct {
	Secret string `json:"secret" binding:"required"`
	Code   string `json:"code" binding:"required"`
}

func (h *AuthHandler) Enable2FA(c *gin.Context) {
	var req enable2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "请求格式错误")
		return
	}

	username := middleware.GetUsername(c)
	if err := h.auth.EnableTOTP(username, req.Secret, req.Code); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	clientIP := c.ClientIP()
	ua := c.GetHeader("User-Agent")
	details := "启用两步验证: " + username
	h.auth.LogSecurityEvent("2FA_ENABLED", clientIP, &ua, &details)
	slog.Info("两步验证已启用", "username", username)

	util.OKMsg(c, "两步验证已启用")
}

type disable2FARequest struct {
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Disable2FA(c *gin.Context) {
	var req disable2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "请求格式错误")
		return
	}

	username := middleware.GetUsername(c)
	if err := h.auth.DisableTOTP(username, req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	clientIP := c.ClientIP()
	ua := c.GetHeader("User-Agent")
	details := "关闭两步验证: " + username
	h.auth.LogSecurityEvent("2FA_DISABLED", clientIP, &ua, &details)
	slog.Info("两步验证已关闭", "username", username)

	util.OKMsg(c, "两步验证已关闭")
}

// --- Token 验证 & 密码修改 ---

func (h *AuthHandler) Validate(c *gin.Context) {
	header := c.GetHeader("Authorization")
	if len(header) < 8 || header[:7] != "Bearer " {
		util.BadRequest(c, "缺少Authorization头部")
		return
	}
	tokenStr := header[7:]
	claims, err := h.auth.ValidateJWT(tokenStr)
	if err != nil {
		util.Unauthorized(c, "令牌无效或已过期")
		return
	}
	util.OK(c, gin.H{
		"subject":   claims["sub"],
		"type":      claims["type"],
		"expiresAt": claims["exp"],
	}, "令牌有效")
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required"`
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "请求格式错误")
		return
	}

	username := middleware.GetUsername(c)
	if username == "" {
		util.Unauthorized(c, "无效的用户身份")
		return
	}

	if err := h.auth.ChangePassword(username, req.CurrentPassword, req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	clientIP := c.ClientIP()
	ua := c.GetHeader("User-Agent")
	details := "管理员密码更改成功: " + username
	h.auth.LogSecurityEvent("PASSWORD_CHANGED", clientIP, &ua, &details)
	slog.Info("管理员密码更改成功", "username", username)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "密码更改成功"})
}
