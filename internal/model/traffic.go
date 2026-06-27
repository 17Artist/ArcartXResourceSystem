// SPDX-License-Identifier: GPL-3.0-or-later

package model

import "time"

type TrafficUsage struct {
	ID            int64     `db:"id" json:"id"`
	Date          string    `db:"date" json:"date"`
	ApiKeyID      *int64    `db:"api_key_id" json:"apiKeyId,omitempty"`
	DownloadCount int       `db:"download_count" json:"downloadCount"`
	TotalBytes    int64     `db:"total_bytes" json:"totalBytes"`
	CreatedAt     time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt     time.Time `db:"updated_at" json:"updatedAt"`
}
