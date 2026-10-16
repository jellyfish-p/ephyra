package rate

import (
	"sync"
	"time"
)

type Limiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func New() *Limiter {
	return &Limiter{hits: map[string][]time.Time{}}
}

func (l *Limiter) Allow(key string, n int, window time.Duration) bool {
	return l.allow(key, n, window, time.Now())
}

func (l *Limiter) allow(key string, n int, window time.Duration, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cut := now.Add(-window)
	cur := l.hits[key]
	i := 0
	for i < len(cur) && cur[i].Before(cut) {
		i++
	}
	cur = cur[i:]
	if len(cur) >= n {
		l.hits[key] = cur
		return false
	}
	l.hits[key] = append(cur, now)
	return true
}
