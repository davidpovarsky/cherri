package main

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/schema"
)

func legacyScopedBlockDiagnostic(name string) string {
	if action, found := schema.DefaultRegistry().LookupAction(name); found && action.ScopedBlock {
		return fmt.Sprintf("E_V2_SCOPED_BLOCK: %s is a V2 scoped block construct; use V2 syntax, or explicit debugStartTimer/debugStopTimer with a saved result in Legacy", name)
	}
	return ""
}
