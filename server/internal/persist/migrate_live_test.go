package persist_test

import (
	"testing"

	"github.com/jellyfish-p/ephyra/server/internal/persist"
	"github.com/jellyfish-p/ephyra/server/internal/testutil"
)

func TestMigrateCreatesCoreTables(t *testing.T) {
	db := testutil.Database(t)
	for _, table := range []string{"accounts", "submissions", "judge_tasks", "judging_attempts", "outbox_events", "schema_migrations"} {
		if err := db.Exec("SELECT 1 FROM " + table + " LIMIT 1").Error; err != nil {
			t.Fatalf("%s: %v", table, err)
		}
	}
	var n int64
	if err := db.Table("schema_migrations").Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatal("no applied migrations")
	}
	if err := persist.Ping(t.Context(), db); err != nil {
		t.Fatal(err)
	}
}
