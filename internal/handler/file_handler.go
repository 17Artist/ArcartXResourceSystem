// SPDX-License-Identifier: GPL-3.0-or-later

package handler

import (
	"arcartx-resource/internal/middleware"
	"arcartx-resource/internal/service"
	"arcartx-resource/internal/util"
	"log/slog"
	"math"

	"github.com/gin-gonic/gin"
)

type FileHandler struct {
	fileSvc    *service.FileService
	linkSvc    *service.SignedLinkService
	trafficSvc *service.TrafficService
	maxSize    int64
}

func NewFileHandler(fileSvc *service.FileService, linkSvc *service.SignedLinkService, trafficSvc *service.TrafficService, maxSize int64) *FileHandler {
	return &FileHandler{
		fileSvc:    fileSvc,
		linkSvc:    linkSvc,
		trafficSvc: trafficSvc,
		maxSize:    maxSize,
	}
}

func (h *FileHandler) SetMaxSize(v int64) { h.maxSize = v }

func (h *FileHandler) List(c *gin.Context) {
	files, err := h.fileSvc.ListFiles()
	if err != nil {
		slog.Error("获取文件列表失败", "error", err)
		util.InternalError(c, "获取文件列表失败")
		return
	}
	totalSize := h.fileSvc.TotalSize()
	util.OK(c, gin.H{
		"files":      files,
		"totalCount": len(files),
		"totalSize":  totalSize,
	}, "")
}

func (h *FileHandler) Upload(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		util.BadRequest(c, "未找到上传文件")
		return
	}
	defer file.Close()

	fileName, fileSize, _, err := h.fileSvc.SaveFile(header.Filename, file, h.maxSize)
	if err != nil {
		slog.Warn("文件上传失败", "file", header.Filename, "error", err)
		util.BadRequest(c, err.Error())
		return
	}

	util.OK(c, gin.H{
		"success":  true,
		"fileName": fileName,
		"fileSize": fileSize,
		"message":  "文件上传成功",
	}, "")
}

func (h *FileHandler) Delete(c *gin.Context) {
	fileName := c.Param("fileName")
	if fileName == "" {
		util.BadRequest(c, "缺少文件名参数")
		return
	}

	if err := h.fileSvc.DeleteFile(fileName); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	util.OKMsg(c, "文件删除成功")
}

func (h *FileHandler) CRC64List(c *gin.Context) {
	files, err := h.fileSvc.GetCRC64List()
	if err != nil {
		slog.Error("获取CRC64列表失败", "error", err)
		util.InternalError(c, "获取文件CRC64列表失败")
		return
	}

	type crc64Item struct {
		FileName string `json:"fileName"`
		CRC64    string `json:"crc64"`
	}
	items := make([]crc64Item, len(files))
	for i, f := range files {
		items[i] = crc64Item{FileName: f.FileName, CRC64: f.CRC64}
	}

	util.OK(c, gin.H{
		"success":    true,
		"files":      items,
		"totalCount": len(items),
		"message":    "文件CRC64列表获取成功",
	}, "")
}

type signedLinkRequest struct {
	FileName          string `json:"fileName" binding:"required"`
	ExpirationMinutes int    `json:"expirationMinutes"`
	DownloadLimit     int    `json:"downloadLimit"`
}

func (h *FileHandler) GenerateSignedLink(c *gin.Context) {
	var req signedLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "请求格式错误")
		return
	}
	if req.ExpirationMinutes <= 0 {
		req.ExpirationMinutes = 30
	}
	if req.DownloadLimit <= 0 {
		req.DownloadLimit = 3
	}

	// 确定创建者
	createdBy := middleware.GetUsername(c)
	if createdBy == "" {
		info := middleware.GetApiKeyInfo(c)
		if info != nil {
			createdBy = info.KeyName
		} else {
			createdBy = "unknown"
		}
	}

	// API Key 请求检查流量
	apiInfo := middleware.GetApiKeyInfo(c)
	if apiInfo != nil {
		fileSize, err := h.fileSvc.GetFileSize(req.FileName)
		if err == nil {
			// 防止 fileSize * downloadLimit 整数溢出绕过流量校验
			estimated := fileSize * int64(req.DownloadLimit)
			if req.DownloadLimit > 0 && estimated/int64(req.DownloadLimit) != fileSize {
				estimated = math.MaxInt64
			}
			if !h.trafficSvc.CheckLimit(estimated) {
				util.TooManyRequests(c, "今日流量限制已达上限，无法生成更多下载链接")
				return
			}
		}
	}

	if !h.fileSvc.FileExists(req.FileName) {
		util.NotFound(c, "文件不存在")
		return
	}

	// 使用 sanitize 后的文件名生成签名链接
	sanitizedName := service.SanitizeFileName(req.FileName)
	link, err := h.linkSvc.Generate(sanitizedName, req.ExpirationMinutes, req.DownloadLimit, createdBy)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	util.OK(c, gin.H{
		"success":       true,
		"downloadUrl":   "/api/download/signed/" + link.Token,
		"expiresAt":     link.ExpiresAt,
		"downloadLimit": link.RemainingDownloads,
	}, "")
}
