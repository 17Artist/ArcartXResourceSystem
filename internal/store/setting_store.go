// SPDX-License-Identifier: GPL-3.0-or-later

package store

import (
	"arcartx-resource/internal/model"
	"time"

	"github.com/jmoiron/sqlx"
)

type SettingStore struct {
	db *sqlx.DB
}

func NewSettingStore(db *sqlx.DB) *SettingStore {
	return &SettingStore{db: db}
}

func (s *SettingStore) GetAll() ([]model.SystemSetting, error) {
	var settings []model.SystemSetting
	err := s.db.Select(&settings, "SELECT * FROM system_settings ORDER BY config_key")
	return settings, err
}

func (s *SettingStore) Get(key string) (*model.SystemSetting, error) {
	var setting model.SystemSetting
	err := s.db.Get(&setting, "SELECT * FROM system_settings WHERE config_key = ?", key)
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

func (s *SettingStore) Upsert(key, value, description string) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(
		`INSERT INTO system_settings (config_key, config_value, description, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(config_key) DO UPDATE SET config_value = ?, updated_at = ?`,
		key, value, description, now, now,
		value, now,
	)
	return err
}

func (s *SettingStore) Update(key, value string) error {
	_, err := s.db.Exec(
		"UPDATE system_settings SET config_value = ?, updated_at = ? WHERE config_key = ?",
		value, time.Now().UTC(), key,
	)
	return err
}

// InitDefaults 初始化默认配置项（不覆盖已有值）
func (s *SettingStore) InitDefaults(defaults map[string]struct{ Value, Desc string }) error {
	now := time.Now().UTC()
	for key, d := range defaults {
		_, err := s.db.Exec(
			`INSERT OR IGNORE INTO system_settings (config_key, config_value, description, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?)`,
			key, d.Value, d.Desc, now, now,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
