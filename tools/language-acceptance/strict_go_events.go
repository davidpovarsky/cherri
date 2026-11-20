package main

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

// Shared closed-world event validator. Exact source-bound scenario inventories
// stay independent; no parent pass can conceal a missing or skipped child.
func parseStrictNamedProductionOutput(names []string, maximum time.Duration, raw []byte) error {
	if len(names) == 0 || maximum <= 0 || !utf8.Valid(raw) {
		return fmt.Errorf("empty production inventory, invalid budget or invalid UTF-8")
	}
	required := map[string]bool{}
	for _, name := range names {
		if name == "" || required[name] {
			return fmt.Errorf("invalid required Go event inventory")
		}
		required[name] = true
	}
	run, passed := map[string]bool{}, map[string]bool{}
	starts, packages := 0, 0
	var previousTime time.Time
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 4<<20)
	for scanner.Scan() {
		if packages != 0 {
			return fmt.Errorf("production output continues after package completion")
		}
		if _, err := decodeNativeEncodingObject(scanner.Bytes()); err != nil {
			return fmt.Errorf("invalid production Go JSON: %w", err)
		}
		var event struct {
			Time, Action, Package, Test, Output string
			Elapsed                             *float64
		}
		if err := decodeEditorJSON(scanner.Bytes(), &event); err != nil {
			return fmt.Errorf("invalid production event: %w", err)
		}
		eventTime, timeErr := time.Parse(time.RFC3339Nano, event.Time)
		if timeErr != nil || eventTime.IsZero() || !previousTime.IsZero() && eventTime.Before(previousTime) {
			return fmt.Errorf("missing, invalid or backwards production event time")
		}
		previousTime = eventTime
		if event.Action == "pass" && event.Elapsed == nil {
			return fmt.Errorf("production pass lacks observed elapsed time")
		}
		if event.Elapsed != nil && (event.Action != "pass" || math.IsNaN(*event.Elapsed) || math.IsInf(*event.Elapsed, 0) || *event.Elapsed < 0 || *event.Elapsed > maximum.Seconds()) {
			return fmt.Errorf("invalid bounded production event elapsed time")
		}
		if event.Package != "github.com/electrikmilk/cherri" || event.Test != "" && !required[event.Test] {
			return fmt.Errorf("foreign or unselected production event %s:%s", event.Package, event.Test)
		}
		switch event.Action {
		case "start":
			if event.Test != "" || starts != 0 || len(run) != 0 {
				return fmt.Errorf("duplicate or reordered production package start")
			}
			starts++
		case "run":
			if starts != 1 || event.Test == "" || run[event.Test] || passed[event.Test] {
				return fmt.Errorf("duplicate or missing production test execution")
			}
			if slash := strings.IndexByte(event.Test, '/'); slash >= 0 {
				parent := event.Test[:slash]
				if !run[parent] || passed[parent] {
					return fmt.Errorf("production child outside its parent execution")
				}
			}
			run[event.Test] = true
		case "pass":
			if event.Test == "" {
				if starts != 1 || len(passed) != len(required) {
					return fmt.Errorf("production package passed without complete scenario")
				}
				packages++
			} else {
				if !run[event.Test] || passed[event.Test] {
					return fmt.Errorf("production pass without unique prior run")
				}
				if !strings.Contains(event.Test, "/") {
					for name := range required {
						if strings.HasPrefix(name, event.Test+"/") && !passed[name] {
							return fmt.Errorf("production parent passed before required child %s", name)
						}
					}
				}
				passed[event.Test] = true
			}
		case "output":
			if starts != 1 || event.Test != "" && (!run[event.Test] || passed[event.Test]) {
				return fmt.Errorf("production output outside active execution")
			}
		default:
			return fmt.Errorf("production test failed, skipped, paused or emitted unknown event %s", event.Action)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if starts != 1 || packages != 1 || len(run) != len(required) || len(passed) != len(required) {
		return fmt.Errorf("incomplete actual production scenario execution")
	}
	return nil
}
