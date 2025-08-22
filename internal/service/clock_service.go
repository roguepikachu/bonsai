package service

import "time"

// Clock is an interface for getting the current time. Useful for testing.
type Clock interface {
	Now() time.Time
}

// RealClock implements Clock using the system time.
type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now()
}
