// SPDX-License-Identifier: GPL-3.0-or-later

package store

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

func New(dbPath string) (*sqlx.DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	db, err := sqlx.Open("sqlite", dbPath+"?_pragma=journal_mode(wal)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(on)")
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	db.SetMaxOpenConns(1) // SQLite 单写
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("数据库连接失败: %w", err)
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}

	slog.Info("数据库初始化完成", "path", dbPath)
	return db, nil
}

func migrate(db *sqlx.DB) error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS admin_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			is_active BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_login_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS api_keys (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key_name TEXT NOT NULL,
			key_hash TEXT NOT NULL UNIQUE,
			key_prefix TEXT NOT NULL DEFAULT '',
			ip_whitelist TEXT,
			is_active BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_used_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS file_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			file_name TEXT NOT NULL UNIQUE,
			file_size INTEGER NOT NULL,
			crc64 TEXT NOT NULL,
			uploaded_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_modified DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS signed_links (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token TEXT NOT NULL UNIQUE,
			file_name TEXT NOT NULL,
			expires_at DATETIME NOT NULL,
			download_limit INTEGER NOT NULL,
			remaining_downloads INTEGER NOT NULL,
			created_by TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS traffic_usage (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date TEXT NOT NULL,
			api_key_id INTEGER REFERENCES api_keys(id),
			download_count INTEGER DEFAULT 0,
			total_bytes INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS security_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			event_type TEXT NOT NULL,
			ip_address TEXT NOT NULL,
			user_agent TEXT,
			user_id INTEGER REFERENCES admin_users(id),
			api_key_id INTEGER REFERENCES api_keys(id),
			details TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS system_settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			config_key TEXT NOT NULL UNIQUE,
			config_value TEXT NOT NULL,
			description TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, ddl := range tables {
		if _, err := db.Exec(ddl); err != nil {
			return fmt.Errorf("执行DDL失败: %w\nSQL: %s", err, ddl)
		}
	}

	// 索引
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_security_logs_created_at ON security_logs(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_traffic_usage_date ON traffic_usage(date)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_traffic_date_key ON traffic_usage(date, IFNULL(api_key_id, -1))`,
		`CREATE INDEX IF NOT EXISTS idx_signed_links_expires_at ON signed_links(expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_signed_links_token ON signed_links(token)`,
	}
	for _, idx := range indexes {
		if _, err := db.Exec(idx); err != nil {
			return fmt.Errorf("创建索引失败: %w", err)
		}
	}

	// 增量迁移：为已有表添加新列（忽略 duplicate column 错误）
	alterations := []string{
		`ALTER TABLE admin_users ADD COLUMN totp_secret TEXT`,
		`ALTER TABLE admin_users ADD COLUMN totp_enabled BOOLEAN DEFAULT 0`,
	}
	for _, alt := range alterations {
		_, _ = db.Exec(alt) // 列已存在时会报错，忽略即可
	}

	return nil
}
