// SPDX-License-Identifier: GPL-3.0-or-later

package store

import (
	"arcartx-resource/internal/model"
	"time"

	"github.com/jmoiron/sqlx"
)

type TrafficStore struct {
	db *sqlx.DB
}

func NewTrafficStore(db *sqlx.DB) *TrafficStore {
	return &TrafficStore{db: db}
}

func (s *TrafficStore) Record(apiKeyID *int64, bytes int64) error {
	today := time.Now().UTC().Format("2006-01-02")
	now := time.Now().UTC()

	_, err := s.db.Exec(
		`INSERT INTO traffic_usage (date, api_key_id, download_count, total_bytes, created_at, updated_at)
		 VALUES (?, ?, 1, ?, ?, ?)
		 ON CONFLICT(date, IFNULL(api_key_id, -1)) DO UPDATE SET
		 download_count = download_count + 1,
		 total_bytes = total_bytes + ?,
		 updated_at = ?`,
		today, apiKeyID, bytes, now, now,
		bytes, now,
	)
	return err
}

func (s *TrafficStore) GetTodayTotal() (int64, int, error) {
	today := time.Now().UTC().Format("2006-01-02")
	var result struct {
		TotalBytes    int64 `db:"total_bytes"`
		DownloadCount int   `db:"download_count"`
	}
	err := s.db.Get(&result,
		"SELECT COALESCE(SUM(total_bytes), 0) as total_bytes, COALESCE(SUM(download_count), 0) as download_count FROM traffic_usage WHERE date = ?",
		today,
	)
	if err != nil {
		return 0, 0, err
	}
	return result.TotalBytes, result.DownloadCount, nil
}

func (s *TrafficStore) GetTodayByKey(apiKeyID int64) (*model.TrafficUsage, error) {
	today := time.Now().UTC().Format("2006-01-02")
	var usage model.TrafficUsage
	err := s.db.Get(&usage, "SELECT * FROM traffic_usage WHERE date = ? AND api_key_id = ?", today, apiKeyID)
	if err != nil {
		return nil, err
	}
	return &usage, nil
}

func (s *TrafficStore) CleanOld(days int) (int64, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02")
	res, err := s.db.Exec("DELETE FROM traffic_usage WHERE date < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
