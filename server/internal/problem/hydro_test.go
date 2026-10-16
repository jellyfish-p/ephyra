package problem

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/jellyfish-p/ephyra/server/internal/domain"
)

func TestImportHydroBatch(t *testing.T) {
	z := zipBytes(t, map[string]string{
		"config.yaml": "type: default\ntime: 1s\nmemory: 256m\nchecker_type: default\nsubtasks:\n  - score: 40\n    cases:\n      - input: 1.in\n        output: 1.out\n  - score: 60\n    cases:\n      - input: 2.in\n        output: 2.out\n",
		"1.in":        "1 2\n",
		"1.out":       "3\n",
		"2.in":        "2 2\n",
		"2.out":       "4\n",
	})
	pkg, err := ImportHydroZip(z)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Config.Kind != domain.KindBatch || pkg.Config.Checker.Kind != domain.CheckerToken {
		t.Fatalf("%+v", pkg.Config)
	}
	if pkg.Config.MaxScore().String() != "100.00" {
		t.Fatal(pkg.Config.MaxScore())
	}
	if pkg.Config.TimeLimitMS != 1000 || pkg.Config.MemoryLimitBytes != 256<<20 {
		t.Fatalf("limits %+v", pkg.Config)
	}
}

func TestImportRejectsUnknownAndPartialScore(t *testing.T) {
	z := zipBytes(t, map[string]string{
		"config.yaml": "type: default\nsubtasks:\n  - score: 100\n    type: min\n    cases:\n      - input: 1.in\n        output: 1.out\n",
		"1.in":        "a",
		"1.out":       "b",
	})
	if _, err := ImportHydroZip(z); err == nil {
		t.Fatal("subtask type min must be rejected")
	}
	z2 := zipBytes(t, map[string]string{
		"config.yaml": "type: submit_answer\ntime: 1s\nmemory: 16m\n",
	})
	if _, err := ImportHydroZip(z2); err == nil {
		t.Fatal("submit_answer must be rejected")
	}
}

func TestImportRejectsPathEscape(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("../secret")
	f.Write([]byte("x"))
	w.Close()
	if _, err := ImportHydroZip(buf.Bytes()); err == nil {
		t.Fatal("path escape")
	}
}

func TestImportInteractive(t *testing.T) {
	z := zipBytes(t, map[string]string{
		"config.yaml":    "type: interactive\ntime: 1000ms\nmemory: 128m\nchecker_type: testlib\nchecker: chk.cpp\ninteractor: interactor.cpp\ncases:\n  - input: 1.in\n",
		"1.in":           "1\n",
		"chk.cpp":        "// checker\n",
		"interactor.cpp": "// interactor\n",
	})
	pkg, err := ImportHydroZip(z)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Config.Kind != domain.KindInteractive || pkg.Config.Interactor == nil {
		t.Fatalf("%+v", pkg.Config)
	}
}

func zipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, body := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
