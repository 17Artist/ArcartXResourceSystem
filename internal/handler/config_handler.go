// SPDX-License-Identifier: GPL-3.0-or-later

package handler

import (
	"arcartx-resource/internal/service"
	"arcartx-resource/internal/store"
	"arcartx-resource/internal/util"
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

var allowedConfigKeys = map[string]bool{
	"daily_traffic_limit":       true,
	"download_rate_limit":       true,
	"login_rate_limit":          true,
	"max_file_size":             true,
	"signed_link_max_minutes":   true,
	"signed_link_max_downloads": true,
}

type ConfigHandler struct {
	settingStore *store.SettingStore
	trafficSvc   *service.TrafficService
	linkSvc      *service.SignedLinkService
	fileHandler  *FileHandler
}

func NewConfigHandler(settingStore *store.SettingStore, trafficSvc *service.TrafficService, linkSvc *service.SignedLinkService, fileHandler *FileHandler) *ConfigHandler {
	return &ConfigHandler{settingStore: settingStore, trafficSvc: trafficSvc, linkSvc: linkSvc, fileHandler: fileHandler}
}

func (h *ConfigHandler) List(c *gin.Context) {
	configs, err := h.settingStore.GetAll()
	if err != nil {
		slog.Error("获取系统配置失败", "error", err)
		util.InternalError(c, "获取系统配置失败")
		return
	}

	type configItem struct {
		Key         string `json:"key"`
		Value       string `json:"value"`
		Description string `json:"description"`
	}
	items := make([]configItem, len(configs))
	for i, cfg := range configs {
		desc := ""
		if cfg.Description != nil {
			desc = *cfg.Description
		}
		items[i] = configItem{Key: cfg.ConfigKey, Value: cfg.ConfigValue, Description: desc}
	}

	util.OK(c, gin.H{
		"success": true,
		"configs": items,
		"message": "系统配置获取成功",
	}, "")
}

type updateConfigRequest struct {
	Configs map[string]string `json:"configs" binding:"required"`
}

func (h *ConfigHandler) Update(c *gin.Context) {
	var req updateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "请求格式错误")
		return
	}

	for key := range req.Configs {
		if !allowedConfigKeys[key] {
			util.BadRequest(c, "不允许修改的配置项: "+key)
			return
		}
	}

	errors := validateConfigs(req.Configs)
	if len(errors) > 0 {
		util.BadRequest(c, "配置值无效: "+errors[0])
		return
	}

	count := 0
	for key, value := range req.Configs {
		if err := h.settingStore.Update(key, value); err != nil {
			slog.Error("更新配置失败", "key", key, "error", err)
			continue
		}
		count++
		slog.Info("系统配置已更新", "key", key, "value", value)
		h.applyRuntimeConfig(key, value)
	}

	util.OK(c, gin.H{
		"success":      true,
		"message":      "系统配置更新成功",
		"updatedCount": count,
	}, "")
}

func (h *ConfigHandler) applyRuntimeConfig(key, value string) {
	switch key {
	case "daily_traffic_limit":
		if v, err := strconv.ParseInt(value, 10, 64); err == nil {
			h.trafficSvc.SetDailyLimit(v)
			slog.Info("运行时流量限制已更新", "limit", v)
		}
	case "max_file_size":
		if v, err := strconv.ParseInt(value, 10, 64); err == nil {
			h.fileHandler.SetMaxSize(v)
			slog.Info("运行时文件大小限制已更新", "limit", v)
		}
	case "signed_link_max_minutes":
		if v, err := strconv.Atoi(value); err == nil {
			h.linkSvc.SetMaxExpiration(time.Duration(v) * time.Minute)
			slog.Info("运行时签名链接有效期已更新", "minutes", v)
		}
	case "signed_link_max_downloads":
		if v, err := strconv.Atoi(value); err == nil {
			h.linkSvc.SetMaxDownloads(v)
			slog.Info("运行时签名链接下载限制已更新", "limit", v)
		}
	}
}

func validateConfigs(configs map[string]string) []string {
	var errs []string
	for key, value := range configs {
		switch key {
		case "daily_traffic_limit":
			v, err := strconv.ParseInt(value, 10, 64)
			if err != nil || v < 1073741824 {
				errs = append(errs, "每日流量限制最小1GB")
			}
			if v > 1099511627776 {
				errs = append(errs, "每日流量限制最大1TB")
			}
		case "download_rate_limit":
			v, err := strconv.Atoi(value)
			if err != nil || v < 1 || v > 1000 {
				errs = append(errs, "下载速率限制必须在1-1000之间")
			}
		case "login_rate_limit":
			v, err := strconv.Atoi(value)
			if err != nil || v < 1 || v > 100 {
				errs = append(errs, "登录限制必须在1-100之间")
			}
		case "max_file_size":
			v, err := strconv.ParseInt(value, 10, 64)
			if err != nil || v < 1048576 {
				errs = append(errs, "文件大小限制最小1MB")
			}
			if v > 2147483648 {
				errs = append(errs, "文件大小限制最大2GB")
			}
		case "signed_link_max_minutes":
			v, err := strconv.Atoi(value)
			if err != nil || v < 1 || v > 1440 {
				errs = append(errs, "链接有效期必须在1-1440分钟之间")
			}
		case "signed_link_max_downloads":
			v, err := strconv.Atoi(value)
			if err != nil || v < 1 || v > 100 {
				errs = append(errs, "下载次数限制必须在1-100之间")
			}
		}
	}
	return errs
}
