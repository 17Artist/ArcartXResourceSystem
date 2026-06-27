// SPDX-License-Identifier: GPL-3.0-or-later

package service

import (
	"arcartx-resource/config"
	"arcartx-resource/internal/model"
	"arcartx-resource/internal/store"
	"arcartx-resource/internal/util"
	"errors"
	"fmt"
	"hash/crc64"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var crc64Table = crc64.MakeTable(crc64.ECMA)

// Windows 保留文件名
var windowsReserved = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true,
	"COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true,
	"LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

type FileService struct {
	cfg       *config.Config
	fileStore *store.FileStore
	uploadDir string
	allowed   map[string]bool
}

func NewFileService(cfg *config.Config, fileStore *store.FileStore) *FileService {
	dir := cfg.Storage.UploadDir
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("创建上传目录失败", "error", err)
	}

	allowed := make(map[string]bool)
	for _, ext := range cfg.Storage.AllowedExtensions {
		allowed[strings.ToLower(ext)] = true
	}

	return &FileService{
		cfg:       cfg,
		fileStore: fileStore,
		uploadDir: dir,
		allowed:   allowed,
	}
}

var unsafeChars = regexp.MustCompile(`[/\\:*?"<>|\x00-\x1f]`)

// SanitizeFileName 清理文件名（导出供其他包使用）
func SanitizeFileName(name string) string {
	// 替换不安全字符
	name = unsafeChars.ReplaceAllString(name, "_")
	// 替换路径遍历（循环直至不再包含 ".."，防止 "...."→".." 残留）
	for strings.Contains(name, "..") {
		name = strings.ReplaceAll(name, "..", "_")
	}
	// 去除前导点（隐藏文件）
	name = strings.TrimLeft(name, ".")
	// 去除尾部空格和点（Windows 会静默截断）
	name = strings.TrimRight(name, " .")
	name = strings.TrimSpace(name)

	if name == "" {
		name = "unnamed_file"
	}

	// 检查 Windows 保留名称
	baseName := name
	if idx := strings.LastIndex(baseName, "."); idx > 0 {
		baseName = baseName[:idx]
	}
	if windowsReserved[strings.ToUpper(baseName)] {
		name = "_" + name
	}

	// 限制文件名长度（255 字节）
	if len(name) > 255 {
		ext := filepath.Ext(name)
		name = name[:255-len(ext)] + ext
	}

	return name
}

// SaveFile 流式保存文件，同时计算 CRC64。返回文件名、大小、CRC64。
func (s *FileService) SaveFile(originalName string, reader io.Reader, maxSize int64) (string, int64, string, error) {
	ext := strings.ToLower(filepath.Ext(originalName))
	if ext != "" {
		ext = ext[1:]
	}
	if !s.allowed[ext] {
		return "", 0, "", fmt.Errorf("不支持的文件类型: %s", ext)
	}

	cleanName := SanitizeFileName(originalName)
	destPath, err := s.resolvePath(cleanName)
	if err != nil {
		return "", 0, "", err
	}

	if _, err := os.Stat(destPath); err == nil {
		return "", 0, "", fmt.Errorf("文件已存在: %s", cleanName)
	}

	dst, err := os.Create(destPath)
	if err != nil {
		slog.Error("创建文件失败", "file", cleanName, "error", err)
		return "", 0, "", errors.New("服务器创建文件失败")
	}
	defer dst.Close()

	hasher := crc64.New(crc64Table)
	written, err := io.Copy(io.MultiWriter(dst, hasher), io.LimitReader(reader, maxSize+1))
	if err != nil {
		os.Remove(destPath)
		slog.Error("写入文件失败", "file", cleanName, "error", err)
		return "", 0, "", errors.New("文件写入失败")
	}

	if written > maxSize {
		os.Remove(destPath)
		return "", 0, "", fmt.Errorf("文件过大，最大支持 %d MB", maxSize/(1024*1024))
	}

	crc64Hex := util.FormatCRC64(hasher.Sum64())

	if err := s.fileStore.Upsert(cleanName, written, crc64Hex); err != nil {
		slog.Error("保存文件记录失败", "file", cleanName, "error", err)
	}

	slog.Info("文件上传成功", "file", cleanName, "size", written, "crc64", crc64Hex)
	return cleanName, written, crc64Hex, nil
}

// resolvePath 清理文件名并返回上传目录内的绝对/相对安全路径。
// 若解析结果逃逸出上传目录则返回错误（纵深防御）。
func (s *FileService) resolvePath(fileName string) (string, error) {
	cleanName := SanitizeFileName(fileName)
	filePath := filepath.Join(s.uploadDir, cleanName)

	absUpload, err1 := filepath.Abs(s.uploadDir)
	absDest, err2 := filepath.Abs(filePath)
	if err1 != nil || err2 != nil {
		return "", errors.New("非法的文件路径")
	}
	if absDest != absUpload && !strings.HasPrefix(absDest, absUpload+string(filepath.Separator)) {
		return "", errors.New("非法的文件路径")
	}
	return filePath, nil
}

// DeleteFile 删除文件
func (s *FileService) DeleteFile(fileName string) error {
	cleanName := SanitizeFileName(fileName)
	filePath, err := s.resolvePath(cleanName)
	if err != nil {
		return err
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("文件不存在: %s", cleanName)
	}

	if err := os.Remove(filePath); err != nil {
		slog.Error("删除文件失败", "file", cleanName, "error", err)
		return errors.New("文件删除失败")
	}

	if err := s.fileStore.Delete(cleanName); err != nil {
		slog.Error("删除文件记录失败", "file", cleanName, "error", err)
	}

	slog.Info("文件删除成功", "file", cleanName)
	return nil
}

func (s *FileService) ListFiles() ([]model.FileRecord, error) {
	return s.fileStore.GetAll()
}

func (s *FileService) GetCRC64List() ([]model.FileRecord, error) {
	return s.fileStore.GetAll()
}

func (s *FileService) FileExists(fileName string) bool {
	filePath, err := s.resolvePath(fileName)
	if err != nil {
		return false
	}
	_, err = os.Stat(filePath)
	return err == nil
}

func (s *FileService) GetFilePath(fileName string) (string, error) {
	filePath, err := s.resolvePath(fileName)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", fmt.Errorf("文件不存在: %s", SanitizeFileName(fileName))
	}
	return filePath, nil
}

func (s *FileService) GetFileSize(fileName string) (int64, error) {
	filePath, err := s.resolvePath(fileName)
	if err != nil {
		return 0, err
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func (s *FileService) TotalSize() int64 {
	files, err := s.fileStore.GetAll()
	if err != nil {
		return 0
	}
	var total int64
	for _, f := range files {
		total += f.FileSize
	}
	return total
}
