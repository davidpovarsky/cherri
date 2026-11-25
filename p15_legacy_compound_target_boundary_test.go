package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestP15LegacyCompoundUnloweredTargetBoundary(t *testing.T) {
	previousMode, previousVersion := embeddedCompilerMode, iosVersion
	embeddedCompilerMode, iosVersion = true, 17
	defer func() { embeddedCompilerMode, iosVersion = previousMode, previousVersion }()
	before := len(shortcut.WFWorkflowActions)
	defer func() {
		failure := recover()
		if failure == nil || !strings.Contains(fmt.Sprint(failure), "E_CONDITION_TARGET") {
			t.Fatalf("unlowered old-target compound must fail before dropping predicates: %v", failure)
		}
		if len(shortcut.WFWorkflowActions) != before {
			t.Fatal("invalid compound emitted a partial native action")
		}
	}()
	makeConditionalAction(&token{typeof: Conditional, ident: "unlowered-target", valueType: If, value: WFConditions{
		conditions:                    []condition{{condition: 4}, {condition: 4}},
		WFActionParameterFilterPrefix: conditionFilterPrefixes[And],
	}})
}
