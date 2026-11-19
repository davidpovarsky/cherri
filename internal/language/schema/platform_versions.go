package schema

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// PlatformVersionBounds records only explicitly supplied version facts for one
// platform. Empty or absent bounds are unknown, not universal availability.
type PlatformVersionBounds struct {
	MinVersion string `json:"minVersion,omitempty"`
	MaxVersion string `json:"maxVersion,omitempty"`
}

// ValidVersionComponents is the strict boundary used for authored target
// versions and canonical per-platform facts. The existing comparator retains
// its permissive legacy behavior; untrusted new bounds must pass this first.
func ValidVersionComponents(value string) bool {
	for _, component := range strings.Split(value, ".") {
		if component == "" {
			return false
		}
		for _, digit := range component {
			if digit < '0' || digit > '9' {
				return false
			}
		}
		if _, err := strconv.Atoi(component); err != nil {
			return false
		}
	}
	return true
}

// CanonicalTargetPlatform retains the existing source spelling alias without
// deriving a platform from another platform or introducing a platform catalog.
func CanonicalTargetPlatform(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "mac" {
		return "macos"
	}
	return value
}

func ValidatePlatformVersionMetadata(facets map[string]PlatformVersionBounds) error {
	platforms := make([]string, 0, len(facets))
	for platform := range facets {
		platforms = append(platforms, platform)
	}
	sort.Strings(platforms)
	for _, platform := range platforms {
		if platform == "" || platform != CanonicalTargetPlatform(platform) {
			return fmt.Errorf("platform version key %q is not a canonical platform name", platform)
		}
		bounds := facets[platform]
		for _, bound := range []struct{ name, value string }{{"minimum", bounds.MinVersion}, {"maximum", bounds.MaxVersion}} {
			if bound.value != "" && !ValidVersionComponents(bound.value) {
				return fmt.Errorf("invalid %s version %q for %s", bound.name, bound.value, platform)
			}
		}
		if bounds.MinVersion != "" && bounds.MaxVersion != "" && CompareVersions(bounds.MinVersion, bounds.MaxVersion) > 0 {
			return fmt.Errorf("minimum version %s exceeds maximum version %s for %s", bounds.MinVersion, bounds.MaxVersion, platform)
		}
	}
	return nil
}

const (
	PlatformVersionUnsupported = "unsupported"
	PlatformVersionUnverified  = "unverified"
	PlatformVersionInvalid     = "invalid-metadata"
	PlatformVersionBadTarget   = "invalid-target"
)

// PlatformVersionIssue is a compiler decision, not an Apple availability claim.
// Analyzer/service and canonical emission consume the same decision function.
type PlatformVersionIssue struct {
	Kind     string
	Platform string
	Message  string
}

// CheckPlatformVersions never falls back from an absent requested-platform
// facet to another platform or to generic legacy float metadata. Callers retain
// the old generic path only when the entire optional facet map is absent.
func CheckPlatformVersions(facets map[string]PlatformVersionBounds, targets []string, minimum, maximum string) []PlatformVersionIssue {
	if len(facets) == 0 {
		return nil
	}
	if err := ValidatePlatformVersionMetadata(facets); err != nil {
		return []PlatformVersionIssue{{Kind: PlatformVersionInvalid, Message: err.Error()}}
	}
	for _, bound := range []string{minimum, maximum} {
		if bound != "" && !ValidVersionComponents(bound) {
			return []PlatformVersionIssue{{Kind: PlatformVersionBadTarget, Message: fmt.Sprintf("invalid target version %q", bound)}}
		}
	}
	if minimum != "" && maximum != "" && CompareVersions(minimum, maximum) > 0 {
		return []PlatformVersionIssue{{Kind: PlatformVersionBadTarget, Message: fmt.Sprintf("target minimum %s exceeds target maximum %s", minimum, maximum)}}
	}
	if len(targets) == 0 {
		return []PlatformVersionIssue{{Kind: PlatformVersionUnverified, Message: "platform-specific version facts require an explicit target platform"}}
	}

	var issues []PlatformVersionIssue
	seen := make(map[string]bool)
	for _, rawTarget := range targets {
		target := CanonicalTargetPlatform(rawTarget)
		if seen[target] {
			continue
		}
		seen[target] = true
		add := func(kind, message string) {
			issues = append(issues, PlatformVersionIssue{Kind: kind, Platform: target, Message: message})
		}
		bounds, found := facets[target]
		if !found || (bounds.MinVersion == "" && bounds.MaxVersion == "") {
			add(PlatformVersionUnverified, fmt.Sprintf("version availability is unverified for %s: no version bounds are recorded for that platform", target))
			continue
		}
		if minimum == "" && maximum == "" {
			add(PlatformVersionUnverified, fmt.Sprintf("version availability is unverified for %s: no target version was specified", target))
			continue
		}
		if bounds.MinVersion != "" {
			switch {
			case minimum != "" && CompareVersions(minimum, bounds.MinVersion) < 0:
				add(PlatformVersionUnsupported, fmt.Sprintf("requires minimum version %s on %s, but target minimum is %s", bounds.MinVersion, target, minimum))
			case minimum == "" && maximum != "" && CompareVersions(maximum, bounds.MinVersion) < 0:
				add(PlatformVersionUnsupported, fmt.Sprintf("requires minimum version %s on %s, but target maximum is %s", bounds.MinVersion, target, maximum))
			case minimum == "":
				add(PlatformVersionUnverified, fmt.Sprintf("minimum-version compatibility is unverified for %s: target minimum is unspecified", target))
			}
		}
		if bounds.MaxVersion != "" {
			switch {
			case maximum != "" && CompareVersions(maximum, bounds.MaxVersion) > 0:
				add(PlatformVersionUnsupported, fmt.Sprintf("is unsupported above version %s on %s, but target maximum is %s", bounds.MaxVersion, target, maximum))
			case maximum == "" && minimum != "" && CompareVersions(minimum, bounds.MaxVersion) > 0:
				add(PlatformVersionUnsupported, fmt.Sprintf("is unsupported above version %s on %s, but target minimum is %s", bounds.MaxVersion, target, minimum))
			case maximum == "":
				add(PlatformVersionUnverified, fmt.Sprintf("maximum-version compatibility is unverified for %s: target maximum is unspecified", target))
			}
		}
	}
	return issues
}
