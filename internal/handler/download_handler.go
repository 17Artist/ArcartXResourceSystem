// SPDX-License-Identifier: GPL-3.0-or-later

package handler

import (
	"arcartx-resource/internal/service"
	"log/slog"
	"mime"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type DownloadHandler struct {
	linkSvc    *service.SignedLinkService
	fileSvc    *service.FileService
	trafficSvc *service.TrafficService
}

func NewDownloadHandler(linkSvc *service.SignedLinkService, fileSvc *service.FileService, trafficSvc *service.TrafficService) *DownloadHandler {
	return &DownloadHandler{
		linkSvc:    linkSvc,
		fileSvc:    fileSvc,
		trafficSvc: trafficSvc,
	}
}

func (h *DownloadHandler) SignedDownload(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.String(http.StatusBadRequest, "无效的请求")
		return
	}

	clientIP := c.ClientIP()

	// 1. 先做只读预检查（廉价拒绝，不消费次数）
	link, err := h.linkSvc.Validate(token)
	if err != nil {
		slog.Warn("签名下载验证失败", "ip", clientIP, "error", err)
		c.String(http.StatusUnauthorized, "下载链接无效或已过期")
		return
	}

	// 2. 检查文件是否存在
	filePath, err := h.fileSvc.GetFilePath(link.FileName)
	if err != nil {
		slog.Error("签名下载文件不存在", "file", link.FileName)
		c.String(http.StatusNotFound, "文件不存在")
		return
	}

	fileSize, _ := h.fileSvc.GetFileSize(link.FileName)

	// 3. 检查流量限制
	if !h.trafficSvc.CheckLimit(fileSize) {
		slog.Warn("每日流量限制已达上限", "file", link.FileName, "ip", clientIP)
		c.String(http.StatusTooManyRequests, "今日流量已达上限，请明日再试")
		return
	}

	// 4. 原子地验证并消费下载次数（防止并发超额下载的竞态）
	link, err = h.linkSvc.ValidateAndConsume(token)
	if err != nil {
		slog.Warn("签名下载消费失败", "ip", clientIP, "error", err)
		c.String(http.StatusUnauthorized, "下载链接无效或已过期")
		return
	}

	slog.Info("签名下载", "file", link.FileName, "size", fileSize, "ip", clientIP)

	// 安全的 Content-Disposition 头（RFC 5987 编码）
	c.Header("Content-Disposition", safeContentDisposition(link.FileName))

	// 使用自定义 ResponseWriter 追踪实际传输字节数
	tw := &trackingWriter{ResponseWriter: c.Writer, written: 0}
	c.Writer = tw

	c.File(filePath)

	// 记录实际传输的流量（在传输完成后）
	if tw.written > 0 {
		h.trafficSvc.Record(nil, tw.written)
	}
}

// safeContentDisposition 生成安全的 Content-Disposition 头
// 使用 RFC 5987 编码处理特殊字符
func safeContentDisposition(fileName string) string {
	// 清理文件名中的控制字符和引号
	safe := strings.Map(func(r rune) rune {
		if r < 32 || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, fileName)

	// 使用 mime.FormatMediaType 安全编码
	params := map[string]string{"filename": safe}
	disposition := mime.FormatMediaType("attachment", params)
	if disposition == "" {
		// fallback
		return "attachment; filename=\"download\""
	}
	return disposition
}

// trackingWriter 包装 gin.ResponseWriter 追踪实际写入字节数
type trackingWriter struct {
	gin.ResponseWriter
	written int64
}

func (tw *trackingWriter) Write(data []byte) (int, error) {
	n, err := tw.ResponseWriter.Write(data)
	tw.written += int64(n)
	return n, err
}
