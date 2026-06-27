// SPDX-License-Identifier: GPL-3.0-or-later

package model

import "time"

type SystemSetting struct {
	ID          int64     `db:"id" json:"id"`
	ConfigKey   string    `db:"config_key" json:"key"`
	ConfigValue string    `db:"config_value" json:"value"`
	Description *string   `db:"description" json:"description,omitempty"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time `db:"updated_at" json:"updatedAt"`
}

type SignedLink struct {
	ID                 int64     `db:"id" json:"id"`
	Token              string    `db:"token" json:"token"`
	FileName           string    `db:"file_name" json:"fileName"`
	ExpiresAt          time.Time `db:"expires_at" json:"expiresAt"`
	DownloadLimit      int       `db:"download_limit" json:"downloadLimit"`
	RemainingDownloads int       `db:"remaining_downloads" json:"remainingDownloads"`
	CreatedBy          string    `db:"created_by" json:"createdBy"`
	CreatedAt          time.Time `db:"created_at" json:"createdAt"`
}
