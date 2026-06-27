// SPDX-License-Identifier: GPL-3.0-or-later

package model

import "time"

type AdminUser struct {
	ID           int64      `db:"id" json:"id"`
	Username     string     `db:"username" json:"username"`
	PasswordHash string     `db:"password_hash" json:"-"`
	IsActive     bool       `db:"is_active" json:"isActive"`
	TOTPSecret   *string    `db:"totp_secret" json:"-"`
	TOTPEnabled  bool       `db:"totp_enabled" json:"totpEnabled"`
	CreatedAt    time.Time  `db:"created_at" json:"createdAt"`
	LastLoginAt  *time.Time `db:"last_login_at" json:"lastLoginAt,omitempty"`
}
