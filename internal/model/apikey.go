// SPDX-License-Identifier: GPL-3.0-or-later

package model

import "time"

type ApiKey struct {
	ID          int64      `db:"id" json:"id"`
	KeyName     string     `db:"key_name" json:"keyName"`
	KeyHash     string     `db:"key_hash" json:"-"`
	KeyPrefix   string     `db:"key_prefix" json:"keyPrefix"`
	IPWhitelist *string    `db:"ip_whitelist" json:"-"`
	IsActive    bool       `db:"is_active" json:"isActive"`
	CreatedAt   time.Time  `db:"created_at" json:"createdAt"`
	LastUsedAt  *time.Time `db:"last_used_at" json:"lastUsedAt,omitempty"`
}

// ApiKeyInfo 用于内存缓存的精简结构
type ApiKeyInfo struct {
	ID          int64
	KeyName     string
	IPWhitelist []string
	IsActive    bool
}
