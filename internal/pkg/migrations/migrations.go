package migrations

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"

	"gorm.io/gorm"
)

//go:embed sql/*.sql
var files embed.FS

func Run(db *gorm.DB) error {
	if err := db.Exec("CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())").Error; err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := fs.Glob(files, "sql/*.sql")
	if err != nil {
		return fmt.Errorf("glob migrations: %w", err)
	}
	sort.Strings(entries)

	for _, name := range entries {
		version := name[len("sql/"):]
		if err := apply(db, name, version); err != nil {
			return err
		}
	}
	return nil
}

func apply(db *gorm.DB, name, version string) error {
	var applied int64
	if err := db.Raw("SELECT count(*) FROM schema_migrations WHERE version = ?", version).Scan(&applied).Error; err != nil {
		return fmt.Errorf("check migration %s: %w", version, err)
	}
	if applied > 0 {
		return nil
	}

	body, err := files.ReadFile(name)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", version, err)
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(string(body)).Error; err != nil {
			return fmt.Errorf("apply migration %s: %w", version, err)
		}
		if err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version).Error; err != nil {
			return fmt.Errorf("record migration %s: %w", version, err)
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}
