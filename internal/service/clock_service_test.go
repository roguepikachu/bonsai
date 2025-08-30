package service

import (
	"testing"
	"time"
)

// TestRealClockNow ensures RealClock returns the current time (within a small delta).
func TestRealClockNow(t *testing.T) {
	rc := RealClock{}
	before := time.Now()
	got := rc.Now()
	after := time.Now()

	if got.Before(before) || got.After(after.Add(50*time.Millisecond)) {
		t.Fatalf("RealClock.Now out of expected range: before=%v got=%v after=%v", before, got, after)
	}
}
