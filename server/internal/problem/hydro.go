package problem

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jellyfish-p/ephyra/server/internal/apperr"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/exact"
	"gopkg.in/yaml.v3"
)

var numberedIn = regexp.MustCompile(`^(.+)\.in$`)

var allowedHydroKeys = map[string]struct{}{
	"type": {}, "filename": {}, "time": {}, "memory": {},
	"checker_type": {}, "checker": {}, "interactor": {},
	"subtasks": {}, "cases": {},
}

var allowedSubtaskKeys = map[string]struct{}{
	"id": {}, "score": {}, "cases": {},
}

var allowedCaseKeys = map[string]struct{}{
	"input": {}, "output": {}, "time": {}, "memory": {},
}

type File struct {
	Name string
	Data []byte
}

type HydroPackage struct {
	Config NormalizedConfig
	Files  []File
}

func ImportHydroZip(raw []byte) (HydroPackage, error) {
	files, err := safeUnzip(raw)
	if err != nil {
		return HydroPackage{}, err
	}
	cfgName, cfgBody, err := findConfig(files)
	if err != nil {
		return HydroPackage{}, err
	}
	ncfg, err := parseHydroConfig(cfgBody, fileNames(files))
	if err != nil {
		return HydroPackage{}, err
	}
	if err := ncfg.Validate(); err != nil {
		return HydroPackage{}, apperr.New(apperr.Invalid, err.Error())
	}
	out := make([]File, 0, len(files))
	for _, f := range files {
		if f.Name == cfgName {
			continue
		}
		out = append(out, f)
	}
	return HydroPackage{Config: ncfg, Files: out}, nil
}

func safeUnzip(raw []byte) ([]File, error) {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, apperr.New(apperr.Invalid, "not a zip archive")
	}
	if len(zr.File) > domain.MaxZipFiles {
		return nil, apperr.New(apperr.Invalid, "too many files in archive")
	}
	var total int64
	var files []File
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		name := path.Clean("/" + strings.ReplaceAll(zf.Name, "\\", "/"))
		name = strings.TrimPrefix(name, "/")
		if name == "" || strings.HasPrefix(name, "..") || strings.Contains(name, ":") {
			return nil, apperr.New(apperr.Invalid, "illegal path in archive")
		}
		if zf.Mode()&0o120000 == 0o120000 { // symlink
			return nil, apperr.New(apperr.Invalid, "symlinks are not allowed")
		}
		if zf.UncompressedSize64 > domain.MaxZipFileBytes {
			return nil, apperr.New(apperr.Invalid, "file too large")
		}
		rc, err := zf.Open()
		if err != nil {
			return nil, apperr.New(apperr.Invalid, "cannot read archive entry")
		}
		limited := io.LimitReader(rc, int64(domain.MaxZipFileBytes)+1)
		data, err := io.ReadAll(limited)
		rc.Close()
		if err != nil {
			return nil, apperr.New(apperr.Invalid, "cannot read archive entry")
		}
		if int64(len(data)) > domain.MaxZipFileBytes {
			return nil, apperr.New(apperr.Invalid, "file too large")
		}
		total += int64(len(data))
		if total > domain.MaxUncompressedZip {
			return nil, apperr.New(apperr.Invalid, "uncompressed archive too large")
		}
		files = append(files, File{Name: name, Data: data})
	}
	if len(files) == 0 {
		return nil, apperr.New(apperr.Invalid, "empty archive")
	}
	return files, nil
}

func findConfig(files []File) (string, []byte, error) {
	var ini bool
	for _, f := range files {
		base := path.Base(f.Name)
		if base == "config.yaml" || base == "config.yml" {
			return f.Name, f.Data, nil
		}
		if base == "config.ini" {
			ini = true
		}
	}
	if ini {
		return "", nil, apperr.New(apperr.Invalid, "config.ini is not supported")
	}
	return "", nil, apperr.New(apperr.Invalid, "config.yaml is required")
}

func fileNames(files []File) map[string]struct{} {
	m := make(map[string]struct{}, len(files))
	for _, f := range files {
		m[path.Base(f.Name)] = struct{}{}
		m[f.Name] = struct{}{}
	}
	return m
}

func parseHydroConfig(body []byte, names map[string]struct{}) (NormalizedConfig, error) {
	var raw map[string]any
	if err := yaml.Unmarshal(body, &raw); err != nil {
		return NormalizedConfig{}, apperr.New(apperr.Invalid, "invalid yaml")
	}
	if err := rejectUnknown(raw, allowedHydroKeys); err != nil {
		return NormalizedConfig{}, err
	}
	typ, _ := raw["type"].(string)
	if typ == "" {
		typ = "default"
	}
	cfg := NormalizedConfig{Version: ConfigVersion, TimeLimitMS: 1000, MemoryLimitBytes: 256 << 20}
	switch typ {
	case "default":
		cfg.Kind = domain.KindBatch
	case "interactive":
		cfg.Kind = domain.KindInteractive
	default:
		return NormalizedConfig{}, apperr.New(apperr.Invalid, fmt.Sprintf("unsupported problem type %q", typ))
	}

	if _, ok := raw["time"]; !ok {
		return NormalizedConfig{}, apperr.New(apperr.Invalid, "time is required")
	}
	ms, err := parseTimeLimit(raw["time"])
	if err != nil {
		return NormalizedConfig{}, err
	}
	cfg.TimeLimitMS = ms
	if _, ok := raw["memory"]; !ok {
		return NormalizedConfig{}, apperr.New(apperr.Invalid, "memory is required")
	}
	mem, err := parseMemory(raw["memory"])
	if err != nil {
		return NormalizedConfig{}, err
	}
	cfg.MemoryLimitBytes = mem

	checkerType, _ := raw["checker_type"].(string)
	if checkerType == "" {
		checkerType = "default"
	}
	checkerFile, _ := raw["checker"].(string)
	switch checkerType {
	case "default":
		cfg.Checker = CheckerSpec{Kind: domain.CheckerToken}
	case "strict":
		cfg.Checker = CheckerSpec{Kind: domain.CheckerExact}
	case "testlib":
		if checkerFile == "" {
			return NormalizedConfig{}, apperr.New(apperr.Invalid, "testlib checker file is required")
		}
		cfg.Checker = CheckerSpec{Kind: domain.CheckerTestlib, SourceName: path.Base(checkerFile)}
	default:
		return NormalizedConfig{}, apperr.New(apperr.Invalid, fmt.Sprintf("unsupported checker_type %q", checkerType))
	}

	if inter, ok := raw["interactor"].(string); ok && inter != "" {
		cfg.Interactor = &InteractorSpec{SourceName: path.Base(inter)}
	}
	if cfg.Kind == domain.KindInteractive && cfg.Interactor == nil {
		return NormalizedConfig{}, apperr.New(apperr.Invalid, "interactive problems require interactor")
	}

	var cases []NormalizedCase
	if rawCases, ok := raw["cases"]; ok {
		parsed, err := parseCases(rawCases)
		if err != nil {
			return NormalizedConfig{}, err
		}
		cases = parsed
	}
	if rawSubs, ok := raw["subtasks"]; ok {
		subs, subCases, err := parseSubtasks(rawSubs)
		if err != nil {
			return NormalizedConfig{}, err
		}
		if len(cases) == 0 {
			cases = subCases
		}
		cfg.Subtasks = subs
		cfg.Cases = mergeCases(cases, subCases)
	} else {
		if len(cases) == 0 {
			auto, err := detectCases(names, cfg.Kind)
			if err != nil {
				return NormalizedConfig{}, err
			}
			cases = auto
		}
		cfg.Cases = cases
		ids := make([]string, 0, len(cases))
		for _, c := range cases {
			ids = append(ids, c.ID)
		}
		cfg.Subtasks = []NormalizedSubtask{{ID: "1", Score: exact.FromInt(100), CaseIDs: ids}}
	}
	return cfg, nil
}

func mergeCases(a, b []NormalizedCase) []NormalizedCase {
	seen := map[string]NormalizedCase{}
	var order []string
	add := func(list []NormalizedCase) {
		for _, c := range list {
			if _, ok := seen[c.ID]; !ok {
				order = append(order, c.ID)
			}
			seen[c.ID] = c
		}
	}
	add(a)
	add(b)
	out := make([]NormalizedCase, 0, len(order))
	for _, id := range order {
		out = append(out, seen[id])
	}
	return out
}

func parseSubtasks(v any) ([]NormalizedSubtask, []NormalizedCase, error) {
	list, ok := v.([]any)
	if !ok {
		return nil, nil, apperr.New(apperr.Invalid, "subtasks must be a list")
	}
	var subs []NormalizedSubtask
	var cases []NormalizedCase
	for i, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, nil, apperr.New(apperr.Invalid, "invalid subtask")
		}
		if err := rejectUnknown(m, allowedSubtaskKeys); err != nil {
			return nil, nil, err
		}
		id := fmt.Sprintf("%d", i+1)
		if s, ok := m["id"]; ok {
			id = fmt.Sprint(s)
		}
		score, err := parseScoreValue(m["score"])
		if err != nil {
			return nil, nil, err
		}
		cs, err := parseCases(m["cases"])
		if err != nil {
			return nil, nil, err
		}
		ids := make([]string, 0, len(cs))
		for _, c := range cs {
			ids = append(ids, c.ID)
			cases = append(cases, c)
		}
		subs = append(subs, NormalizedSubtask{ID: id, Score: score, CaseIDs: ids})
	}
	return subs, cases, nil
}

func parseCases(v any) ([]NormalizedCase, error) {
	if v == nil {
		return nil, apperr.New(apperr.Invalid, "cases required")
	}
	list, ok := v.([]any)
	if !ok {
		return nil, apperr.New(apperr.Invalid, "cases must be a list")
	}
	var out []NormalizedCase
	for i, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, apperr.New(apperr.Invalid, "invalid case")
		}
		if err := rejectUnknown(m, allowedCaseKeys); err != nil {
			return nil, err
		}
		input, _ := m["input"].(string)
		output, _ := m["output"].(string)
		if input == "" {
			return nil, apperr.New(apperr.Invalid, "case input is required")
		}
		id := strings.TrimSuffix(path.Base(input), path.Ext(input))
		if id == "" {
			id = strconv.Itoa(i + 1)
		}
		c := NormalizedCase{ID: id, InputName: path.Base(input), OutputName: path.Base(output)}
		if v, ok := m["time"]; ok {
			ms, err := parseTimeLimit(v)
			if err != nil {
				return nil, err
			}
			c.TimeLimitMS = &ms
		}
		if v, ok := m["memory"]; ok {
			b, err := parseMemory(v)
			if err != nil {
				return nil, err
			}
			c.MemoryLimitBytes = &b
		}
		out = append(out, c)
	}
	return out, nil
}

func detectCases(names map[string]struct{}, kind domain.ProblemKind) ([]NormalizedCase, error) {
	var bases []string
	seen := map[string]struct{}{}
	for name := range names {
		base := path.Base(name)
		m := numberedIn.FindStringSubmatch(base)
		if m == nil {
			continue
		}
		if _, ok := seen[m[1]]; ok {
			continue
		}
		seen[m[1]] = struct{}{}
		bases = append(bases, m[1])
	}
	sort.Strings(bases)
	if len(bases) == 0 {
		return nil, apperr.New(apperr.Invalid, "no test cases found")
	}
	var out []NormalizedCase
	for _, b := range bases {
		c := NormalizedCase{ID: b, InputName: b + ".in"}
		outName := b + ".out"
		if _, ok := names[outName]; ok {
			c.OutputName = outName
		} else if kind == domain.KindBatch {
			return nil, apperr.New(apperr.Invalid, "missing output for "+c.InputName)
		}
		out = append(out, c)
	}
	return out, nil
}

func parseScoreValue(v any) (exact.Score, error) {
	switch t := v.(type) {
	case nil:
		return exact.Score{}, apperr.New(apperr.Invalid, "subtask score required")
	case int:
		return exact.FromInt(int64(t)), nil
	case int64:
		return exact.FromInt(t), nil
	case uint64:
		return exact.FromInt(int64(t)), nil
	case float64:
		return exact.Score{}, apperr.New(apperr.Invalid, "subtask score must not be a binary float; use a decimal string or integer")
	case string:
		s, err := exact.Parse(t)
		if err != nil {
			return exact.Score{}, apperr.New(apperr.Invalid, err.Error())
		}
		return s, nil
	default:
		return exact.Score{}, apperr.New(apperr.Invalid, "invalid subtask score")
	}
}

func parseTimeLimit(v any) (int64, error) {
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(strings.ToLower(t))
		switch {
		case strings.HasSuffix(s, "ms"):
			n, err := strconv.ParseInt(strings.TrimSuffix(s, "ms"), 10, 64)
			if err != nil || n <= 0 {
				return 0, apperr.New(apperr.Invalid, "invalid time limit")
			}
			return n, nil
		case strings.HasSuffix(s, "s"):
			n, err := strconv.ParseInt(strings.TrimSuffix(s, "s"), 10, 64)
			if err != nil || n <= 0 {
				return 0, apperr.New(apperr.Invalid, "invalid time limit")
			}
			return n * 1000, nil
		default:
			return 0, apperr.New(apperr.Invalid, "time limit must use ms or s units")
		}
	default:
		return 0, apperr.New(apperr.Invalid, "time limit must be a string with units")
	}
}

func parseMemory(v any) (int64, error) {
	s, ok := v.(string)
	if !ok {
		return 0, apperr.New(apperr.Invalid, "memory limit must be a string with units")
	}
	s = strings.TrimSpace(strings.ToLower(s))
	mult := int64(1)
	switch {
	case strings.HasSuffix(s, "gib") || strings.HasSuffix(s, "gb"):
		mult = 1 << 30
		s = strings.TrimSuffix(strings.TrimSuffix(s, "gib"), "gb")
	case strings.HasSuffix(s, "mib") || strings.HasSuffix(s, "mb") || strings.HasSuffix(s, "m"):
		mult = 1 << 20
		s = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(s, "mib"), "mb"), "m")
	case strings.HasSuffix(s, "kib") || strings.HasSuffix(s, "kb") || strings.HasSuffix(s, "k"):
		mult = 1 << 10
		s = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(s, "kib"), "kb"), "k")
	default:
		return 0, apperr.New(apperr.Invalid, "memory limit must use k/m/g units")
	}
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || n <= 0 {
		return 0, apperr.New(apperr.Invalid, "invalid memory limit")
	}
	return n * mult, nil
}

func rejectUnknown(m map[string]any, allowed map[string]struct{}) error {
	for k := range m {
		if _, ok := allowed[k]; !ok {
			return apperr.New(apperr.Invalid, fmt.Sprintf("unsupported config key %q", k))
		}
	}
	return nil
}
