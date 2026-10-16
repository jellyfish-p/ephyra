package judging

import (
	"sort"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/apperr"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
)

type PoolSnapshot struct {
	ID             uuid.UUID
	Priority       int
	Enabled        bool
	Offline        bool
	HasProfile     bool
	HealthyNodes   int
	FreeSlots      int
}

type RouteDecision struct {
	PoolID uuid.UUID
	Wait   bool
}

// SelectPool applies organization routing: same priority load-balances by
// free slots; a busy but healthy level is waited on, not skipped; only
// unavailable levels fall through. Exhaustion is a scheduling error.
func SelectPool(pools []PoolSnapshot) (RouteDecision, error) {
	if len(pools) == 0 {
		return RouteDecision{}, apperr.New(apperr.Unavailable, "no routing pools configured")
	}
	byPri := map[int][]PoolSnapshot{}
	var pris []int
	for _, p := range pools {
		if _, ok := byPri[p.Priority]; !ok {
			pris = append(pris, p.Priority)
		}
		byPri[p.Priority] = append(byPri[p.Priority], p)
	}
	sort.Slice(pris, func(i, j int) bool { return pris[i] > pris[j] })

	for _, pri := range pris {
		level := byPri[pri]
		var candidates []PoolSnapshot
		levelBusy := false
		levelUnavailable := true
		for _, p := range level {
			if !p.Enabled || p.Offline || !p.HasProfile || p.HealthyNodes == 0 {
				continue
			}
			levelUnavailable = false
			if p.FreeSlots > 0 {
				candidates = append(candidates, p)
			} else {
				levelBusy = true
			}
		}
		if len(candidates) > 0 {
			sort.Slice(candidates, func(i, j int) bool {
				if candidates[i].FreeSlots != candidates[j].FreeSlots {
					return candidates[i].FreeSlots > candidates[j].FreeSlots
				}
				return candidates[i].ID.String() < candidates[j].ID.String()
			})
			return RouteDecision{PoolID: candidates[0].ID}, nil
		}
		if !levelUnavailable && levelBusy {
			return RouteDecision{Wait: true}, nil
		}
	}
	return RouteDecision{}, apperr.New(apperr.Unavailable, "no capable pool available")
}

func SlotAvailable(maxSlots, occupied int) bool {
	if maxSlots < 1 {
		maxSlots = domain.DefaultMaxSlots
	}
	return occupied < maxSlots
}

func NodeLost(enabled bool, heartbeatAge, lostAfter int64) bool {
	if !enabled {
		return true
	}
	return heartbeatAge > lostAfter
}
