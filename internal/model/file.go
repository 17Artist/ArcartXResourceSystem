// SPDX-License-Identifier: GPL-3.0-or-later

package model

import "time"

type FileRecord struct {
	ID           int64     `db:"id" json:"id"`
	FileName     string    `db:"file_name" json:"fileName"`
	FileSize     int64     `db:"file_size" json:"fileSize"`
	CRC64        string    `db:"crc64" json:"crc64"`
	UploadedAt   time.Time `db:"uploaded_at" json:"uploadedAt"`
	LastModified time.Time `db:"last_modified" json:"lastModified"`
}
