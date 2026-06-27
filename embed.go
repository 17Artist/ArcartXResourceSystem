// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"embed"
	"io/fs"
	"log"
)

//go:embed web/static/*
var staticFS embed.FS

func getStaticFS() fs.FS {
	sub, err := fs.Sub(staticFS, "web/static")
	if err != nil {
		log.Fatalf("加载嵌入式静态资源失败: %v", err)
	}
	return sub
}
