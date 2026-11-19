//go:build !windows

package main

import (
	"testing"
	"time"
)

func p15EditorMeasurementClock(t *testing.T) p15EditorClock {
	t.Helper()
	origin := time.Now()
	return p15EditorClock{
		Name:        "Go monotonic time.Since; microseconds; nearest-rank percentiles",
		FrequencyHz: 1_000_000_000,
		Now:         func() int64 { return time.Since(origin).Nanoseconds() },
	}
}
