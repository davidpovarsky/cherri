package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func p15CacheDateSource(legacy bool, value string, ttl int) string {
	if legacy {
		return fmt.Sprintf("#define version 27\n#include 'actions/calendar'\nconst cached = cache(\"P15_DATE_KEY\", %d) {\ntext(\"P15_DATE_BODY_ONCE\")\nyield %s\n}\ngetVariable(cached)\n", ttl, value)
	}
	return fmt.Sprintf("shortcut \"Date cache\" { version:\"27\" }\nlet cached=cache(key:\"P15_DATE_KEY\",ttl:%d){\ntext(\"P15_DATE_BODY_ONCE\")\nyield %s\n}\nreturn cached\n", ttl, value)
}

func p15CacheDateWireError(actions []ShortcutAction, persistent bool) error {
	var dateGuard, restores, stores, reads, body int
	var guardIndex, storeIndex = -1, -1
	for index, action := range actions {
		var p = action.WFWorkflowActionParameters
		switch action.WFWorkflowActionIdentifier {
		case "is.workflow.actions.gettext":
			if p["WFTextActionText"] == "P15_DATE_BODY_ONCE" {
				body++
			}
		case "is.workflow.actions.getstoredcontent", "is.workflow.actions.setstoredcontent":
			var key, okay = p["WFStoredContentKey"].(string)
			if !okay || !strings.HasPrefix(key, "cherri.cache.v1.local.date.") {
				return fmt.Errorf("Date reused another type namespace")
			}
			if action.WFWorkflowActionIdentifier == "is.workflow.actions.getstoredcontent" {
				reads++
			} else {
				stores++
				storeIndex = index
			}
		case "is.workflow.actions.adjustdate":
			restores++
			if p["WFAdjustOperation"] != "Add" {
				return fmt.Errorf("Date reconstruction changed operation")
			}
			var q = debugParamMap(p["WFDuration"])
			var v = debugParamMap(q["Value"])
			if q["WFSerializationType"] != "WFQuantityFieldValue" || v["Unit"] != "sec" {
				return fmt.Errorf("Date lost exact seconds quantity")
			}
			var magnitude = debugParamMap(v["Magnitude"])
			if magnitude["Type"] != "ActionOutput" || magnitude["WFSerializationType"] != nil || magnitude["Value"] != nil {
				return fmt.Errorf("Date seconds must use the proven bare quantity reference")
			}
			var uuid, okay = magnitude["OutputUUID"].(string)
			if !okay || uuid == "" {
				return fmt.Errorf("Date seconds lost original offset reference")
			}

		case "is.workflow.actions.conditional":
			if _, found := p["WFDate"]; found {
				dateGuard++
				guardIndex = index
				if fmt.Sprint(p["WFCondition"]) != "5" {
					return fmt.Errorf("Date reconstruction equality was weakened")
				}
				if _, wrong := p["WFNumberValue"]; wrong {
					return fmt.Errorf("Date equality uses numeric field")
				}
				if _, wrong := p["WFConditionalActionString"]; wrong {
					return fmt.Errorf("Date equality uses textual field")
				}
			}
		}
	}
	if body != 1 {
		return fmt.Errorf("Date body count=%d", body)
	}
	if !persistent {
		if reads != 0 || stores != 0 || restores != 0 || dateGuard != 0 {
			return fmt.Errorf("TTL0 retained Date archival helpers")
		}
		return nil
	}
	if reads != 1 || stores != 1 || restores != 2 || dateGuard != 1 || guardIndex >= storeIndex {
		return fmt.Errorf("Date archive chain incomplete: reads=%d stores=%d restores=%d dateGuard=%d", reads, stores, restores, dateGuard)
	}
	return nil
}

func TestP15CacheDatePersistenceGraphs(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			for _, tc := range []struct{ name, value string }{
				{"before_epoch", "date(\"1969-12-31T23:59:59Z\")"},
				{"fractional", "date(\"2026-11-27T14:00:00.125Z\")"},
				{"current", "CurrentDate"},
			} {
				t.Run(fmt.Sprintf("%s/legacy=%t/%s", tc.name, legacy, profile), func(t *testing.T) {
					var actions, err = p15CacheCompileRecord(t, p15CacheDateSource(legacy, tc.value, 300), legacy, profile)
					if err != nil {
						t.Fatal(err)
					}
					if err = p15CacheDateWireError(actions, true); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func TestP15CacheDateZeroTTLGraphs(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy=%t", legacy), func(t *testing.T) {
			var actions, err = p15CacheCompileRecord(t, p15CacheDateSource(legacy, "CurrentDate", 0), legacy, BuildProfileRelease)
			if err != nil {
				t.Fatal(err)
			}
			if err = p15CacheDateWireError(actions, false); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15CacheDateOracleRejectsLossyArchiveMutations(t *testing.T) {
	var actions, err = p15CacheCompileRecord(t, p15CacheDateSource(false, "date(\"2026-11-27T14:00:00.125Z\")", 300), false, BuildProfileRelease)
	if err != nil {
		t.Fatal(err)
	}
	if err = p15CacheDateWireError(actions, true); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"namespace", "quantity_unit", "date_equality"} {
		t.Run(name, func(t *testing.T) {
			for _, action := range actions {
				var p = action.WFWorkflowActionParameters
				if name == "namespace" && action.WFWorkflowActionIdentifier == "is.workflow.actions.getstoredcontent" {
					var original = p["WFStoredContentKey"]
					p["WFStoredContentKey"] = "cherri.cache.v1.local.number.changed"
					defer func() { p["WFStoredContentKey"] = original }()
					break
				}
				if name == "quantity_unit" && action.WFWorkflowActionIdentifier == "is.workflow.actions.adjustdate" {
					var q = debugParamMap(p["WFDuration"])
					var v = debugParamMap(q["Value"])
					var original = v["Unit"]
					v["Unit"] = "days"
					defer func() { v["Unit"] = original }()
					break
				}
				if name == "date_equality" && action.WFWorkflowActionIdentifier == "is.workflow.actions.conditional" {
					if _, found := p["WFDate"]; found {
						var original = p["WFCondition"]
						p["WFCondition"] = 4
						defer func() { p["WFCondition"] = original }()
						break
					}
				}
			}
			if err = p15CacheDateWireError(actions, true); err == nil {
				t.Fatal("lossy cache corruption escaped oracle")
			}
		})
	}
}

func TestP15CacheDateNativeProbeSourcesCompile(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy=%t", legacy), func(t *testing.T) {
			var dialect = "v2"
			if legacy {
				dialect = "legacy"
			}
			var content, err = os.ReadFile("tests/language-v2/cache-date-native-" + dialect + ".cherri")
			if err != nil {
				t.Fatal(err)
			}
			var actions, compileErr = p15CacheCompileRecord(t, string(content), legacy, BuildProfileRelease)
			if compileErr != nil {
				t.Fatal(compileErr)
			}
			if len(p15ActionsNamed(actions, "adjustdate")) != 8 {
				t.Fatal("native probe does not exercise four real Date cache calls")
			}
		})
	}
}

func TestP15CacheDateSourceCoercions(t *testing.T) {
	for _, tc := range []struct{ name, setup, expression, kind, contentClass, referenceKind string }{
		{"current_date", "", "CurrentDate.date", "date", "WFDateContentItem", "CurrentDate"},
		{"current_text", "", "CurrentDate.text", "text", "WFStringContentItem", "CurrentDate"},
		{"action_date", "let source=text(\"2026-11-27T14:00:00.125Z\")", "source.date", "date", "WFDateContentItem", "ActionOutput"},
		{"mutable_date", "var source=text(\"2026-11-27T14:00:00.125Z\")", "source.date", "date", "WFDateContentItem", "Variable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var source = "shortcut \"cache source coercion\" { version:\"27\" }\n" + tc.setup + "\nlet cached=cache(key:\"P15_DATE_COERCION\",ttl:300){yield " + tc.expression + "}\nreturn cached\n"
			var actions, err = p15CacheCompileRecord(t, source, false, BuildProfileRelease)
			if err != nil {
				t.Fatal(err)
			}
			var captures = 0
			for _, action := range actions {
				var params = action.WFWorkflowActionParameters
				if action.WFWorkflowActionIdentifier == "is.workflow.actions.getstoredcontent" {
					if key, ok := params["WFStoredContentKey"].(string); !ok || !strings.HasPrefix(key, "cherri.cache.v1.local."+tc.kind+".") {
						t.Fatal("cache namespace ignored the authored source coercion")
					}
				}
				if action.WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" {
					continue
				}
				var reference = debugParamMap(debugParamMap(params["WFVariable"])["Value"])
				if reference["Type"] != tc.referenceKind {
					continue
				}
				var aggrandizements, ok = reference["Aggrandizements"].([]any)
				if !ok || len(aggrandizements) != 1 {
					continue
				}
				var transform = debugParamMap(aggrandizements[0])
				if transform["Type"] == "WFCoercionVariableAggrandizement" && transform["CoercionItemClass"] == tc.contentClass {
					captures++
				}
			}
			if captures != 1 {
				t.Fatalf("authored source coercion snapshot count=%d, want 1", captures)
			}
		})
	}
}
