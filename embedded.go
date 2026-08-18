/*
 * Copyright (c) Cherri
 */

package main

// embeddedCompilerMode is enabled by in-process hosts such as the iOS bridge.
// The normal CLI never enables this flag, so its existing exit behavior remains
// unchanged.
var embeddedCompilerMode bool

type embeddedCompilerPanic struct {
	message string
}

func (e embeddedCompilerPanic) Error() string {
	return e.message
}
