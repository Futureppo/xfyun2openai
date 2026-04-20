package pool

import "time"

type AppState struct {
	InFlight      int
	FailCount     int
	CooldownUntil time.Time
}

type FinishResult struct {
	Success   bool
	Retryable bool
	Cooldown  bool
}
