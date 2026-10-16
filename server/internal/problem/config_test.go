package problem

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jellyfish-p/ephyra/server/internal/domain"
)

func TestNormalizedConfigV1Fixture(t *testing.T) {
	t.Parallel()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "protocol", "testdata", "normalized_config_v1.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg NormalizedConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.Version != ConfigVersion || cfg.Kind != domain.KindBatch {
		t.Fatalf("%+v", cfg)
	}
}
