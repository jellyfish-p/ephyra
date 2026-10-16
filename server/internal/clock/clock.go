package clock

import "time"

type Clock interface {
	Now() time.Time
}

type System struct{}

func (System) Now() time.Time { return time.Now().UTC() }

type Fixed struct{ T time.Time }

func (f Fixed) Now() time.Time { return f.T.UTC() }

type Controlled struct {
	t time.Time
}

func NewControlled(t time.Time) *Controlled {
	return &Controlled{t: t.UTC()}
}

func (c *Controlled) Now() time.Time { return c.t }

func (c *Controlled) Set(t time.Time) { c.t = t.UTC() }

func (c *Controlled) Advance(d time.Duration) { c.t = c.t.Add(d) }
