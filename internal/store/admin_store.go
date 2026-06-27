// SPDX-License-Identifier: GPL-3.0-or-later

package store

import (
	"arcartx-resource/internal/model"
	"time"

	"github.com/jmoiron/sqlx"
)

type AdminStore struct {
	db *sqlx.DB
}

func NewAdminStore(db *sqlx.DB) *AdminStore {
	return &AdminStore{db: db}
}

func (s *AdminStore) GetByUsername(username string) (*model.AdminUser, error) {
	var user model.AdminUser
	err := s.db.Get(&user, "SELECT * FROM admin_users WHERE username = ? AND is_active = 1", username)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *AdminStore) Create(username, passwordHash string) error {
	_, err := s.db.Exec(
		"INSERT INTO admin_users (username, password_hash, is_active, created_at) VALUES (?, ?, 1, ?)",
		username, passwordHash, time.Now().UTC(),
	)
	return err
}

func (s *AdminStore) UpdatePassword(username, passwordHash string) error {
	_, err := s.db.Exec(
		"UPDATE admin_users SET password_hash = ? WHERE username = ? AND is_active = 1",
		passwordHash, username,
	)
	return err
}

func (s *AdminStore) UpdateLastLogin(username string) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(
		"UPDATE admin_users SET last_login_at = ? WHERE username = ?",
		now, username,
	)
	return err
}

func (s *AdminStore) Exists(username string) (bool, error) {
	var count int
	err := s.db.Get(&count, "SELECT COUNT(*) FROM admin_users WHERE username = ?", username)
	return count > 0, err
}

func (s *AdminStore) EnableTOTP(username, secret string) error {
	_, err := s.db.Exec(
		"UPDATE admin_users SET totp_secret = ?, totp_enabled = 1 WHERE username = ? AND is_active = 1",
		secret, username,
	)
	return err
}

func (s *AdminStore) DisableTOTP(username string) error {
	_, err := s.db.Exec(
		"UPDATE admin_users SET totp_secret = NULL, totp_enabled = 0 WHERE username = ? AND is_active = 1",
		username,
	)
	return err
}
