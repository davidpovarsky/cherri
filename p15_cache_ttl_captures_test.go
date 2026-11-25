package main

import (
	"fmt"
	"testing"
)

func p15CacheCapturedSource(legacy bool, ttl int) string {
	if legacy {
		return fmt.Sprintf("#define version 27\n@counter=0\nconst first=cache(\"P15_SCALAR_KEY\",%d){\n@counter=1\nconst counter=9\ntext(\"P15_CACHE_BODY_ONCE\")\nyield counter\n}\n@counter=2\nconst second=cache(\"P15_SCALAR_KEY\",%d){\n@counter=3\ntext(\"P15_CACHE_BODY_ONCE\")\nyield 9\n}\ngetVariable(@counter)\n", ttl, ttl)
	}
	return fmt.Sprintf("shortcut \"cache captures\" {version:\"27\"}\nvar counter=0\nlet first=cache(key:\"P15_SCALAR_KEY\",ttl:%d){counter=1\nlet counter=9\ntext(\"P15_CACHE_BODY_ONCE\")\nyield counter}\ncounter=2\nlet second=cache(key:\"P15_SCALAR_KEY\",ttl:%d){counter=3\ntext(\"P15_CACHE_BODY_ONCE\")\nyield 9}\nreturn counter", ttl, ttl)
}
func TestP15CacheCapturedWritesAndShadowing(t *testing.T) {
	for _, ttl := range []int{0, 300} {
		for _, legacy := range []bool{false, true} {
			for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
				t.Run(fmt.Sprintf("ttl=%d/legacy=%t/%s", ttl, legacy, profile), func(t *testing.T) {
					actual, err := p15CacheCompileRecord(t, p15CacheCapturedSource(legacy, ttl), legacy, profile)
					if err != nil {
						t.Fatal(err)
					}
					effects := 0
					names := map[string]bool{}
					for _, a := range actual {
						if a.WFWorkflowActionIdentifier == "is.workflow.actions.gettext" && a.WFWorkflowActionParameters["WFTextActionText"] == "P15_CACHE_BODY_ONCE" {
							effects++
						}
						if a.WFWorkflowActionIdentifier == "is.workflow.actions.setvariable" {
							if n, okay := a.WFWorkflowActionParameters["WFVariableName"].(string); okay {
								names[n] = true
							}
						}
					}
					if effects != 2 || !names["counter"] {
						t.Fatal("authored captured effects were dropped")
					}
					reads, writes := len(p15ActionsNamed(actual, "getstoredcontent")), len(p15ActionsNamed(actual, "setstoredcontent"))
					if ttl == 0 {
						if reads != 0 || writes != 0 {
							t.Fatal("zero TTL persisted")
						}
					} else {
						if reads != 2 || writes != 2 {
							t.Fatal("two lazy calls lost their independent lifecycle")
						}
						if err := p15CacheNativeWireError(actual); err != nil {
							t.Fatal(err)
						}
					}
				})
			}
		}
	}
}
