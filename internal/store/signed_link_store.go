// SPDX-License-Identifier: GPL-3.0-or-later

package store

import (
	"arcartx-resource/internal/model"
	"time"

	"github.com/jmoiron/sqlx"
)

type SignedLinkStore struct {
	db *sqlx.DB
}

func NewSignedLinkStore(db *sqlx.DB) *SignedLinkStore {
	return &SignedLinkStore{db: db}
}

func (s *SignedLinkStore) Create(link *model.SignedLink) error {
	_, err := s.db.Exec(
		`INSERT INTO signed_links (token, file_name, expires_at, download_limit, remaining_downloads, created_by, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		link.Token, link.FileName, link.ExpiresAt, link.DownloadLimit, link.RemainingDownloads, link.CreatedBy, link.CreatedAt,
	)
	return err
}

func (s *SignedLinkStore) GetByToken(token string) (*model.SignedLink, error) {
	var link model.SignedLink
	err := s.db.Get(&link, "SELECT * FROM signed_links WHERE token = ?", token)
	if err != nil {
		return nil, err
	}
	return &link, nil
}

func (s *SignedLinkStore) DecrementDownloads(token string) error {
	_, err := s.db.Exec(
		"UPDATE signed_links SET remaining_downloads = remaining_downloads - 1 WHERE token = ? AND remaining_downloads > 0",
		token,
	)
	return err
}

func (s *SignedLinkStore) Delete(token string) error {
	_, err := s.db.Exec("DELETE FROM signed_links WHERE token = ?", token)
	return err
}

func (s *SignedLinkStore) LoadActive() ([]model.SignedLink, error) {
	var links []model.SignedLink
	err := s.db.Select(&links,
		"SELECT * FROM signed_links WHERE expires_at > ? AND remaining_downloads > 0",
		time.Now().UTC(),
	)
	return links, err
}

func (s *SignedLinkStore) CleanExpired() (int64, error) {
	res, err := s.db.Exec(
		"DELETE FROM signed_links WHERE expires_at <= ? OR remaining_downloads <= 0",
		time.Now().UTC(),
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *SignedLinkStore) Count() (int, error) {
	var count int
	err := s.db.Get(&count, "SELECT COUNT(*) FROM signed_links WHERE expires_at > ? AND remaining_downloads > 0", time.Now().UTC())
	return count, err
}
