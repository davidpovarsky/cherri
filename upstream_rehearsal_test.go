package main

import "testing"

// The rehearsal executes the real generator and compiles its result. Its
// temporary source replica keeps the caller's worktree and parallel tests intact.
func TestUpstreamPropagationRehearsal(t *testing.T) {
	p15UpstreamProductionRehearsal(t)
}
