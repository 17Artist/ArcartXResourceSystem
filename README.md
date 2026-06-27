# ArcartX 资源后端

> 独立部署的资源更新后端，配合游戏端 **ArcartX 插件**实现资源文件的上传、托管与按需下发。

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](./LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8.svg)](https://go.dev/)

## 配置

复制模板后按需修改，未提供配置文件时使用内置默认值：

```bash
cp config.example.yaml config.yaml
```

关键项：监听端口、JWT 密钥与有效期、上传目录与单文件上限、CORS 来源、各级限流、签名链接上限、每日流量上限。完整说明见 [config.example.yaml](./config.example.yaml) 与部署手册。

> 生产环境务必：修改默认密码、启用两步验证、将 `cors_origins` 改为实际域名、为 API 密钥设置 IP 白名单。

## API 概览

| 方法     | 路径                                | 认证      | 说明            |
|--------|-----------------------------------|---------|---------------|
| `GET`  | `/api/files/crc64-list`           | API Key | 获取文件 CRC64 清单 |
| `POST` | `/api/files/generate-signed-link` | API Key | 生成签名下载链接      |
| `GET`  | `/api/download/signed/:token`     | 签名      | 通过签名链接下载      |



## 许可证

本项目以 **GNU General Public License v3.0** 开源，详见 [LICENSE](./LICENSE)。

```
ArcartX 资源后端
Copyright (C) 2026 17Artist

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.
```
