package stream

// ApplyEventResult is how a Redis Streams delivery is applied. Postgres
// already holds the task; the stream must not mint a judging attempt.
type ApplyEventResult struct {
	CreateAttempt bool
	RunSchedule   bool
}

func ApplyEvent(firstSeen bool) ApplyEventResult {
	return ApplyEventResult{
		CreateAttempt: false,
		RunSchedule:   true,
	}
}
