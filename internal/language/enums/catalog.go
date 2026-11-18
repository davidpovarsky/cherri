// Package enums resolves source and catalog enum names to their exact wire values.
// Native enum spellings are preserved; no identifier aliases are invented.
package enums

import (
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

type Member struct {
	Name      string
	WireValue string
}

type Catalog map[string][]Member

func New(registry *schema.Registry, program *syntax.Program) Catalog {
	catalog := make(Catalog)
	if registry != nil {
		for name, values := range registry.AllEnums() {
			catalog[name] = make([]Member, 0, len(values))
			for _, value := range values {
				catalog[name] = append(catalog[name], Member{Name: value, WireValue: value})
			}
		}
		// Dynamically supplied action schemas may carry their complete enum
		// domain on ParameterSchema without a separate Registry.AllEnums entry.
		// Union those explicit wire values into the shared catalog; each call
		// still validates against its own parameter's narrower EnumValues.
		for _, action := range registry.AllActions() {
			for _, parameter := range action.Parameters {
				if parameter.EnumName == "" {
					continue
				}
				for _, value := range parameter.EnumValues {
					found := false
					for _, member := range catalog[parameter.EnumName] {
						if member.Name == value {
							found = true
							break
						}
					}
					if !found {
						catalog[parameter.EnumName] = append(catalog[parameter.EnumName], Member{Name: value, WireValue: value})
					}
				}
			}
		}
	}
	if program != nil {
		for _, declaration := range program.Declarations {
			if declaration, ok := declaration.(*syntax.EnumDecl); ok {
				members := make([]Member, 0, len(declaration.Members))
				for _, member := range declaration.Members {
					members = append(members, Member{Name: member.Name, WireValue: member.WireValue})
				}
				catalog[declaration.Name] = members
			}
		}
	}
	return catalog
}

func (c Catalog) Has(name string) bool { _, ok := c[name]; return ok }

func (c Catalog) Resolve(name, member string) (string, bool) {
	for _, candidate := range c[name] {
		if candidate.Name == member {
			return candidate.WireValue, true
		}
	}
	return "", false
}

func (c Catalog) Values(name string) []string {
	var values []string
	for _, member := range c[name] {
		values = append(values, member.WireValue)
	}
	return values
}

// TypeName recognizes one enum domain, including an optional annotation.
// Union/container annotations do not imply a single contextual member domain.
func (c Catalog) TypeName(annotation syntax.TypeAnnotation) string {
	switch annotation := annotation.(type) {
	case *syntax.NamedTypeAnnotation:
		if annotation.Namespace == "" && c.Has(annotation.Name) {
			return annotation.Name
		}
	case *syntax.OptionalTypeAnnotation:
		return c.TypeName(annotation.Inner)
	}
	return ""
}
