// SPDX-License-Identifier: GPL-3.0-or-later

package store

import (
	"time"

	"github.com/jmoiron/sqlx"
)

type SecurityStore struct {
	db *sqlx.DB
}

func NewSecurityStore(db *sqlx.DB) *SecurityStore {
	return &SecurityStore{db: db}
}

func (s *SecurityStore) Log(eventType, ipAddress string, userAgent *string, userID, apiKeyID *int64, details *string) error {
	_, err := s.db.Exec(
		`INSERT INTO security_logs (event_type, ip_address, user_agent, user_id, api_key_id, details, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		eventType, ipAddress, userAgent, userID, apiKeyID, details, time.Now().UTC(),
	)
	return err
}

func (s *SecurityStore) CleanOld(days int) (int64, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	res, err := s.db.Exec("DELETE FROM security_logs WHERE created_at < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
