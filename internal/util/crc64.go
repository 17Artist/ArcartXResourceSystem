// SPDX-License-Identifier: GPL-3.0-or-later

package util

import (
	"hash/crc64"
	"io"
)

var crc64Table = crc64.MakeTable(crc64.ECMA)

// NewCRC64Writer 返回一个 io.Writer，写入数据的同时计算 CRC64
func NewCRC64Writer() *CRC64Writer {
	return &CRC64Writer{
		hash: crc64.New(crc64Table),
	}
}

type CRC64Writer struct {
	hash io.Writer
	sum  uint64
	h    interface{ Sum64() uint64 }
}

func init() {
	// 确保类型断言可用
}

// CRC64Hash 封装 crc64 hash
type CRC64Hash struct {
	h *crc64.Table
}

// ComputeFromReader 从 reader 流式计算 CRC64
func ComputeFromReader(r io.Reader) (uint64, error) {
	h := crc64.New(crc64Table)
	if _, err := io.Copy(h, r); err != nil {
		return 0, err
	}
	return h.Sum64(), nil
}

// FormatCRC64 将 CRC64 值格式化为 16 位十六进制字符串
func FormatCRC64(v uint64) string {
	return sprintf("%016x", v)
}

// 避免导入 fmt 只为一个格式化
func sprintf(format string, v uint64) string {
	const hex = "0123456789abcdef"
	buf := make([]byte, 16)
	for i := 15; i >= 0; i-- {
		buf[i] = hex[v&0xf]
		v >>= 4
	}
	return string(buf)
}
