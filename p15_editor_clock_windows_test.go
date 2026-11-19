//go:build windows

package main

import (
	"syscall"
	"testing"
	"unsafe"
)

// The Windows Go runtime's interrupt-time clock can quantize tiny service
// operations to zero. QPC measures each individual operation without batching.
func p15EditorMeasurementClock(t *testing.T) p15EditorClock {
	t.Helper()
	kernel := syscall.NewLazyDLL("kernel32.dll")
	frequencyQuery := kernel.NewProc("QueryPerformanceFrequency")
	counterQuery := kernel.NewProc("QueryPerformanceCounter")
	var frequency int64
	if ok, _, err := frequencyQuery.Call(uintptr(unsafe.Pointer(&frequency))); ok == 0 || frequency <= 0 {
		t.Fatalf("QueryPerformanceFrequency failed: %v", err)
	}
	return p15EditorClock{
		Name:        "Windows monotonic QueryPerformanceCounter; microseconds; nearest-rank percentiles",
		FrequencyHz: frequency,
		Now: func() int64 {
			var counter int64
			if ok, _, err := counterQuery.Call(uintptr(unsafe.Pointer(&counter))); ok == 0 {
				t.Fatalf("QueryPerformanceCounter failed: %v", err)
			}
			return counter
		},
	}
}
