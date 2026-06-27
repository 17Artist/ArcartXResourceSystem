// SPDX-License-Identifier: GPL-3.0-or-later

package store

import (
	"arcartx-resource/internal/model"
	"time"

	"github.com/jmoiron/sqlx"
)

type FileStore struct {
	db *sqlx.DB
}

func NewFileStore(db *sqlx.DB) *FileStore {
	return &FileStore{db: db}
}

func (s *FileStore) Create(fileName string, fileSize int64, crc64 string) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(
		"INSERT INTO file_records (file_name, file_size, crc64, uploaded_at, last_modified) VALUES (?, ?, ?, ?, ?)",
		fileName, fileSize, crc64, now, now,
	)
	return err
}

func (s *FileStore) Update(fileName string, fileSize int64, crc64 string) error {
	_, err := s.db.Exec(
		"UPDATE file_records SET file_size = ?, crc64 = ?, last_modified = ? WHERE file_name = ?",
		fileSize, crc64, time.Now().UTC(), fileName,
	)
	return err
}

func (s *FileStore) Delete(fileName string) error {
	_, err := s.db.Exec("DELETE FROM file_records WHERE file_name = ?", fileName)
	return err
}

func (s *FileStore) GetAll() ([]model.FileRecord, error) {
	var files []model.FileRecord
	err := s.db.Select(&files, "SELECT * FROM file_records ORDER BY file_name")
	return files, err
}

func (s *FileStore) GetByName(fileName string) (*model.FileRecord, error) {
	var file model.FileRecord
	err := s.db.Get(&file, "SELECT * FROM file_records WHERE file_name = ?", fileName)
	if err != nil {
		return nil, err
	}
	return &file, nil
}

func (s *FileStore) Upsert(fileName string, fileSize int64, crc64 string) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(
		`INSERT INTO file_records (file_name, file_size, crc64, uploaded_at, last_modified) 
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(file_name) DO UPDATE SET file_size = ?, crc64 = ?, last_modified = ?`,
		fileName, fileSize, crc64, now, now,
		fileSize, crc64, now,
	)
	return err
}
