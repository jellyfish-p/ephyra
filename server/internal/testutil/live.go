package testutil

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func Database(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("EPHYRA_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("EPHYRA_TEST_DATABASE_URL not set")
	}
	schema := "t" + strings.ReplaceAll(uuid.NewString(), "-", "")
	bootstrap, err := persist.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = bootstrap.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE").Error
		sqlDB, err := bootstrap.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := bootstrap.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}

	scoped := dsn
	if strings.Contains(scoped, "?") {
		scoped += "&search_path=" + schema
	} else {
		scoped += "?search_path=" + schema
	}
	db, err := persist.Open(scoped)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("SET search_path TO " + schema).Error; err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := persist.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func Redis(t *testing.T) *redis.Client {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv("EPHYRA_TEST_REDIS_URL"))
	if raw == "" {
		t.Skip("EPHYRA_TEST_REDIS_URL not set")
	}
	opt, err := redis.ParseURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}
