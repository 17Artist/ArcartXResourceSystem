// SPDX-License-Identifier: GPL-3.0-or-later

package model

import "time"

type SecurityLog struct {
	ID        int64     `db:"id" json:"id"`
	EventType string    `db:"event_type" json:"eventType"`
	IPAddress string    `db:"ip_address" json:"ipAddress"`
	UserAgent *string   `db:"user_agent" json:"userAgent,omitempty"`
	UserID    *int64    `db:"user_id" json:"userId,omitempty"`
	ApiKeyID  *int64    `db:"api_key_id" json:"apiKeyId,omitempty"`
	Details   *string   `db:"details" json:"details,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
}
