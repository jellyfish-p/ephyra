package persist

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/jellyfish-p/ephyra/server/migrations"
)

func TestInitSQLSplits(t *testing.T) {
	body, err := fs.ReadFile(migrations.SQL, "0001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	stmts := splitSQL(string(body))
	if len(stmts) < 20 {
		t.Fatalf("too few statements: %d", len(stmts))
	}
	joined := strings.Join(stmts, "\n")
	for _, needle := range []string{"CREATE TABLE accounts", "CREATE TABLE submissions", "CREATE TABLE judging_attempts", "CREATE TABLE outbox_events"} {
		if !strings.Contains(joined, needle) {
			t.Fatalf("missing %s", needle)
		}
	}
}
