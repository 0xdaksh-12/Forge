package db

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Init opens (or creates) the SQLite database and runs migrations.
func Init(dsn string) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dsn), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	sqlDB, err := database.DB()
	if err != nil {
		return nil, err
	}
	// SQLite is single-writer; avoid "database is locked" errors.
	sqlDB.SetMaxOpenConns(1)

	// Enable WAL mode and foreign keys.
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	} {
		if err := database.Exec(pragma).Error; err != nil {
			return nil, fmt.Errorf("pragma: %w", err)
		}
	}

	if err := database.AutoMigrate(&Pipeline{}, &Secret{}, &Build{}, &Job{}, &LogLine{}); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return database, nil
}
