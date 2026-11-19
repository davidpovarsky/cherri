package analysis

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
)

// A present platform facet map owns the version decision. A missing requested
// platform stays unverified and cannot inherit the generic legacy bound.
func (a *Analyzer) checkPlatformVersions(action *schema.ActionSchema, name string, span source.Span) bool {
	if len(action.PlatformVersions) == 0 {
		return false
	}
	for _, issue := range schema.CheckPlatformVersions(action.PlatformVersions, a.targets, a.minTargetVersionSource, a.maxTargetVersionSource) {
		message := fmt.Sprintf("action %q: %s", name, issue.Message)
		switch issue.Kind {
		case schema.PlatformVersionUnverified:
			a.warn(CodePlatformVersionUnverified, span, message)
		case schema.PlatformVersionInvalid:
			a.error(CodePlatformVersionMetadata, span, message)
		case schema.PlatformVersionBadTarget:
			a.error(CodeTargetValueShape, span, message)
		default:
			a.error(CodeUnsupportedTarget, span, message)
		}
	}
	return true
}
