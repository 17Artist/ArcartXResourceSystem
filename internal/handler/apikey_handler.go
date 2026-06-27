// SPDX-License-Identifier: GPL-3.0-or-later

package handler

import (
	"arcartx-resource/internal/service"
	"arcartx-resource/internal/util"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ApiKeyHandler struct {
	auth *service.AuthService
}

func NewApiKeyHandler(auth *service.AuthService) *ApiKeyHandler {
	return &ApiKeyHandler{auth: auth}
}

func (h *ApiKeyHandler) List(c *gin.Context) {
	keys, err := h.auth.ListApiKeys()
	if err != nil {
		slog.Error("获取API密钥列表失败", "error", err)
		util.InternalError(c, "获取API密钥列表失败")
		return
	}

	type keyItem struct {
		ID        int64  `json:"id"`
		KeyName   string `json:"keyName"`
		KeyPrefix string `json:"keyPrefix"`
		IsActive  bool   `json:"isActive"`
		CreatedAt string `json:"createdAt"`
	}
	items := make([]keyItem, len(keys))
	for i, k := range keys {
		items[i] = keyItem{
			ID:        k.ID,
			KeyName:   k.KeyName,
			KeyPrefix: k.KeyPrefix,
			IsActive:  k.IsActive,
			CreatedAt: k.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}
	util.OK(c, gin.H{"keys": items, "count": len(items)}, "")
}

type createKeyRequest struct {
	Name string `json:"name" binding:"required"`
}

func (h *ApiKeyHandler) Create(c *gin.Context) {
	var req createKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "请求格式错误，需要提供name字段")
		return
	}

	apiKey, err := h.auth.CreateApiKey(req.Name)
	if err != nil {
		slog.Error("创建API密钥失败", "error", err)
		util.InternalError(c, "创建API密钥失败")
		return
	}

	slog.Info("API密钥已创建", "name", req.Name)
	util.OK(c, gin.H{
		"success": true,
		"apiKey":  apiKey,
		"message": "API密钥创建成功，请妥善保存，密钥仅显示一次",
	}, "")
}

func (h *ApiKeyHandler) Reset(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		util.BadRequest(c, "无效的ID")
		return
	}

	apiKey, err := h.auth.ResetApiKey(id)
	if err != nil {
		slog.Error("重置API密钥失败", "error", err)
		util.InternalError(c, "重置API密钥失败")
		return
	}

	slog.Info("API密钥已重置", "id", id)
	util.OK(c, gin.H{
		"success": true,
		"apiKey":  apiKey,
		"message": "API密钥重置成功",
	}, "")
}

func (h *ApiKeyHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		util.BadRequest(c, "无效的ID")
		return
	}

	if err := h.auth.DeleteApiKey(id); err != nil {
		slog.Error("删除API密钥失败", "error", err)
		util.InternalError(c, "删除API密钥失败")
		return
	}

	slog.Info("API密钥已删除", "id", id)
	util.OKMsg(c, "API密钥已删除")
}

func (h *ApiKeyHandler) GetWhitelist(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		util.BadRequest(c, "无效的ID")
		return
	}

	ips, err := h.auth.GetApiKeyWhitelist(id)
	if err != nil {
		util.InternalError(c, "获取IP白名单失败")
		return
	}

	util.OK(c, gin.H{"ipWhitelist": ips, "count": len(ips)}, "IP白名单获取成功")
}

type whitelistRequest struct {
	IPWhitelist []string `json:"ipWhitelist"`
}

func (h *ApiKeyHandler) UpdateWhitelist(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		util.BadRequest(c, "无效的ID")
		return
	}

	var req whitelistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "请求格式错误")
		return
	}

	// 验证 IP 格式
	for _, ip := range req.IPWhitelist {
		if !util.ValidateIP(ip) {
			util.BadRequest(c, "无效的IP地址: "+ip)
			return
		}
	}

	if err := h.auth.UpdateApiKeyWhitelist(id, req.IPWhitelist); err != nil {
		util.InternalError(c, "更新IP白名单失败")
		return
	}

	slog.Info("IP白名单已更新", "keyId", id)
	util.OKMsg(c, "IP白名单更新成功")
}

func (h *ApiKeyHandler) TrafficStats(c *gin.Context) {
	// 这个由 handler 外部注入 trafficSvc 来处理
	// 在路由注册时直接绑定
}
