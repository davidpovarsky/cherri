package analysis

import (
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// Registry is the effective catalog for the most recently analyzed program.
// Callers must treat it as read-only and pass it to consumers of that program.
func (a *Analyzer) Registry() *schema.Registry { return a.registry }

type analyzerTargetConfig struct {
	isMac                *bool
	targets              []string
	explicit             bool
	min, max             float64
	minSource, maxSource string
}

func (a *Analyzer) captureConfiguredPlatforms() {
	if a.analyzing {
		return
	}
	a.configuredTarget.isMac = cloneTargetBool(a.isMacTarget)
	a.configuredTarget.targets = append([]string(nil), a.targets...)
	a.configuredTarget.explicit = a.hasExplicitTarget
}

func cloneTargetBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	result := *value
	return &result
}

func (a *Analyzer) beginProgram() {
	a.registry = a.baseRegistry.Clone()
	a.diagnostics = nil
	a.exprTypes = make(map[syntax.Expression]types.Type)
	a.resolvedActions = make(map[*syntax.CallExpr]*schema.ActionSchema)
	a.currentFn = nil
	a.inValueBlk = false
	a.yieldedTypes = nil
	a.loopDepth = 0
	a.allowedSetupExpression = nil
	a.isMacTarget = cloneTargetBool(a.configuredTarget.isMac)
	a.targets = append([]string(nil), a.configuredTarget.targets...)
	a.hasExplicitTarget = a.configuredTarget.explicit
	a.minTargetVersion, a.maxTargetVersion = a.configuredTarget.min, a.configuredTarget.max
	a.minTargetVersionSource, a.maxTargetVersionSource = a.configuredTarget.minSource, a.configuredTarget.maxSource
	a.analyzing = true
}
