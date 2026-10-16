package problem

import (
	"fmt"

	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/exact"
)

const ConfigVersion = 1

type NormalizedConfig struct {
	Version          int                 `json:"version"`
	Kind             domain.ProblemKind  `json:"kind"`
	Checker          CheckerSpec         `json:"checker"`
	Interactor       *InteractorSpec     `json:"interactor,omitempty"`
	TimeLimitMS      int64               `json:"time_limit_ms"`
	MemoryLimitBytes int64               `json:"memory_limit_bytes"`
	Subtasks         []NormalizedSubtask `json:"subtasks"`
	Cases            []NormalizedCase    `json:"cases"`
}

type CheckerSpec struct {
	Kind       domain.CheckerKind `json:"kind"`
	SourceName string             `json:"source_name,omitempty"`
}

type InteractorSpec struct {
	SourceName string `json:"source_name"`
}

type NormalizedSubtask struct {
	ID      string      `json:"id"`
	Score   exact.Score `json:"score"`
	CaseIDs []string    `json:"case_ids"`
}

type NormalizedCase struct {
	ID               string `json:"id"`
	InputName        string `json:"input_name"`
	OutputName       string `json:"output_name,omitempty"`
	TimeLimitMS      *int64 `json:"time_limit_ms,omitempty"`
	MemoryLimitBytes *int64 `json:"memory_limit_bytes,omitempty"`
}

func (c NormalizedConfig) Validate() error {
	if c.Version != ConfigVersion {
		return fmt.Errorf("unsupported config version %d", c.Version)
	}
	if c.Kind != domain.KindBatch && c.Kind != domain.KindInteractive {
		return fmt.Errorf("unsupported problem kind %q", c.Kind)
	}
	if c.Kind == domain.KindInteractive && (c.Interactor == nil || c.Interactor.SourceName == "") {
		return fmt.Errorf("interactive problems require an interactor")
	}
	if c.Checker.Kind == domain.CheckerTestlib && c.Checker.SourceName == "" {
		return fmt.Errorf("testlib checker requires a source file")
	}
	if c.TimeLimitMS <= 0 || c.MemoryLimitBytes <= 0 {
		return fmt.Errorf("time and memory limits must be positive")
	}
	if len(c.Cases) == 0 || len(c.Subtasks) == 0 {
		return fmt.Errorf("at least one case and subtask are required")
	}
	seenCase := map[string]struct{}{}
	for _, cs := range c.Cases {
		if cs.ID == "" || cs.InputName == "" {
			return fmt.Errorf("case id and input are required")
		}
		if c.Kind == domain.KindBatch && cs.OutputName == "" && c.Checker.Kind != domain.CheckerTestlib {
			// batch with built-in compare needs output; testlib may still use output
			if c.Checker.Kind == domain.CheckerExact || c.Checker.Kind == domain.CheckerToken {
				return fmt.Errorf("case %s missing output", cs.ID)
			}
		}
		if _, ok := seenCase[cs.ID]; ok {
			return fmt.Errorf("duplicate case %s", cs.ID)
		}
		seenCase[cs.ID] = struct{}{}
	}
	seenSub := map[string]struct{}{}
	used := map[string]struct{}{}
	var sum exact.Score
	for _, st := range c.Subtasks {
		if st.ID == "" {
			return fmt.Errorf("subtask id required")
		}
		if _, ok := seenSub[st.ID]; ok {
			return fmt.Errorf("duplicate subtask %s", st.ID)
		}
		seenSub[st.ID] = struct{}{}
		if len(st.CaseIDs) == 0 {
			return fmt.Errorf("subtask %s has no cases", st.ID)
		}
		for _, id := range st.CaseIDs {
			if _, ok := seenCase[id]; !ok {
				return fmt.Errorf("subtask %s references unknown case %s", st.ID, id)
			}
			used[id] = struct{}{}
		}
		sum = sum.Add(st.Score)
	}
	if len(used) != len(seenCase) {
		return fmt.Errorf("every case must belong to a subtask")
	}
	_ = sum
	return nil
}

func (c NormalizedConfig) MaxScore() exact.Score {
	var s exact.Score
	for _, st := range c.Subtasks {
		s = s.Add(st.Score)
	}
	return s
}
