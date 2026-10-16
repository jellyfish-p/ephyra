package persist

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/migrations"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:  logger.Default.LogMode(logger.Warn),
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)
	return db, nil
}

func Migrate(ctx context.Context, db *gorm.DB) error {
	entries, err := fs.ReadDir(migrations.SQL, ".")
	if err != nil {
		return err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL
		)`).Error; err != nil {
			return err
		}
		applied := map[string]struct{}{}
		rows, err := tx.Raw(`SELECT version FROM schema_migrations`).Rows()
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var v string
			if err := rows.Scan(&v); err != nil {
				return err
			}
			applied[v] = struct{}{}
		}
		for _, name := range files {
			if _, ok := applied[name]; ok {
				continue
			}
			body, err := fs.ReadFile(migrations.SQL, name)
			if err != nil {
				return err
			}
			for _, stmt := range splitSQL(string(body)) {
				if err := tx.Exec(stmt).Error; err != nil {
					return fmt.Errorf("migration %s: %w", name, err)
				}
			}
			if err := tx.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`, name, time.Now().UTC()).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func splitSQL(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ";") {
		stmt := strings.TrimSpace(part)
		if stmt == "" {
			continue
		}
		out = append(out, stmt)
	}
	return out
}

func Audit(tx *gorm.DB, ev AuditEvent) error {
	if ev.ID == uuid.Nil {
		ev.ID = uuid.New()
	}
	if ev.Payload == nil {
		ev.Payload = []byte("{}")
	}
	if ev.CreatedAt.IsZero() {
		ev.CreatedAt = time.Now().UTC()
	}
	return tx.Create(&ev).Error
}
