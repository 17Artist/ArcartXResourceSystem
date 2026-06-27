// SPDX-License-Identifier: GPL-3.0-or-later

package store

import (
	"arcartx-resource/internal/model"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"
)

type ApiKeyStore struct {
	db *sqlx.DB
}

func NewApiKeyStore(db *sqlx.DB) *ApiKeyStore {
	return &ApiKeyStore{db: db}
}

func (s *ApiKeyStore) Create(keyName, keyHash, keyPrefix string) (int64, error) {
	res, err := s.db.Exec(
		"INSERT INTO api_keys (key_name, key_hash, key_prefix, is_active, created_at) VALUES (?, ?, ?, 1, ?)",
		keyName, keyHash, keyPrefix, time.Now().UTC(),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *ApiKeyStore) GetAll() ([]model.ApiKey, error) {
	var keys []model.ApiKey
	err := s.db.Select(&keys, "SELECT * FROM api_keys ORDER BY created_at DESC")
	return keys, err
}

func (s *ApiKeyStore) GetActive() ([]model.ApiKey, error) {
	var keys []model.ApiKey
	err := s.db.Select(&keys, "SELECT * FROM api_keys WHERE is_active = 1 ORDER BY created_at DESC")
	return keys, err
}

func (s *ApiKeyStore) GetByID(id int64) (*model.ApiKey, error) {
	var key model.ApiKey
	err := s.db.Get(&key, "SELECT * FROM api_keys WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	return &key, nil
}

func (s *ApiKeyStore) GetByHash(keyHash string) (*model.ApiKey, error) {
	var key model.ApiKey
	err := s.db.Get(&key, "SELECT * FROM api_keys WHERE key_hash = ? AND is_active = 1", keyHash)
	if err != nil {
		return nil, err
	}
	return &key, nil
}

func (s *ApiKeyStore) UpdateHash(id int64, keyHash, keyPrefix string) error {
	_, err := s.db.Exec(
		"UPDATE api_keys SET key_hash = ?, key_prefix = ?, created_at = ? WHERE id = ?",
		keyHash, keyPrefix, time.Now().UTC(), id,
	)
	return err
}

func (s *ApiKeyStore) UpdateIPWhitelist(id int64, whitelist *string) error {
	_, err := s.db.Exec(
		"UPDATE api_keys SET ip_whitelist = ? WHERE id = ?",
		whitelist, id,
	)
	return err
}

func (s *ApiKeyStore) UpdateLastUsed(id int64) error {
	now := time.Now().UTC()
	_, err := s.db.Exec("UPDATE api_keys SET last_used_at = ? WHERE id = ?", now, id)
	return err
}

func (s *ApiKeyStore) Delete(id int64) error {
	_, err := s.db.Exec("DELETE FROM api_keys WHERE id = ?", id)
	return err
}

func (s *ApiKeyStore) GetByName(name string) (*model.ApiKey, error) {
	var key model.ApiKey
	err := s.db.Get(&key, "SELECT * FROM api_keys WHERE key_name = ? AND is_active = 1", name)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &key, nil
}
