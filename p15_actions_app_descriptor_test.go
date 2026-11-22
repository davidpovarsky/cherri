package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	"howett.net/plist"
)

const p15ActionsAppProducer = "40100000-0000-4000-8000-000000000001"

// The descriptor is observed public Actions 4.0.1 metadata. The workflow,
// comment, UUID and output reference are synthetic transport probes. Neither
// importability nor an installation flag establishes installed runtime support.
func p15ActionsAppObservedRoot(t *testing.T) map[string]any {
	t.Helper()
	var raw, err = os.ReadFile("tests/language-v2/actions-app-4.0.1-location-observation.json")
	if err != nil {
		t.Fatal(err)
	}
	var observed struct {
		App        string         `json:"app"`
		Bundle     string         `json:"bundleIdentifier"`
		Identifier string         `json:"shortcutActionIdentifier"`
		Descriptor map[string]any `json:"appIntentDescriptor"`
		Parameters []any          `json:"parameters"`
	}
	if err := json.Unmarshal(raw, &observed); err != nil {
		t.Fatal(err)
	}
	var descriptor = map[string]any{
		"TeamIdentifier": "YG56YK5RN5", "BundleIdentifier": "com.sindresorhus.Actions",
		"Name": "Actions", "AppIntentIdentifier": "IsLocationServicesEnabledIntent",
		"ActionRequiresAppInstallation": true,
	}
	if observed.App != "Actions" || observed.Bundle != "com.sindresorhus.Actions" ||
		observed.Identifier != "com.sindresorhus.Actions.IsLocationServicesEnabledIntent" ||
		observed.Parameters == nil || len(observed.Parameters) != 0 ||
		!reflect.DeepEqual(descriptor, observed.Descriptor) {
		t.Fatal("the independently fixed observed Actions 4.0.1 descriptor changed")
	}
	var root = baseTestHeader()
	root["WFWorkflowActions"] = []any{
		p15AcceptanceAction("is.workflow.actions.comment", map[string]any{"WFCommentActionText": "Public Actions descriptor transport probe"}),
		p15AcceptanceAction(observed.Identifier, map[string]any{
			"AppIntentDescriptor": observed.Descriptor, "UUID": p15ActionsAppProducer, "CustomOutputName": "Synthetic output",
		}),
		p15AcceptanceAction("is.workflow.actions.getvariable", map[string]any{"WFVariable": p15AcceptanceRef(p15ActionsAppProducer, nil)}),
	}
	return root
}

func p15ActionsAppExactError(expected, actual map[string]any) error {
	// Both sides are independently decoded native trees. Do not normalize the
	// descriptor with the production raw-action converter before comparing it.
	if !reflect.DeepEqual(expected, actual) {
		return fmt.Errorf("Actions App exact native tree changed at %s", firstPlistDiffPath(expected, actual, ""))
	}
	if hasImportProvenanceKey(actual) {
		return fmt.Errorf("Actions App output leaked compiler provenance")
	}
	return nil
}

func p15ActionsAppClone(value any) any {
	switch value := value.(type) {
	case map[string]any:
		var result = make(map[string]any, len(value))
		for key, item := range value {
			result[key] = p15ActionsAppClone(item)
		}
		return result
	case []any:
		var result = make([]any, len(value))
		for index, item := range value {
			result[index] = p15ActionsAppClone(item)
		}
		return result
	case []byte:
		return append([]byte(nil), value...)
	default:
		return value
	}
}

func TestP15ActionsAppObservedDescriptorImportAndNegatives(t *testing.T) {
	var root = p15ActionsAppObservedRoot(t)
	var original = buildNativePlistBytes(t, root, plist.XMLFormat)
	var expected map[string]any
	if _, err := plist.Unmarshal(original, &expected); err != nil {
		t.Fatal(err)
	}
	var observations = p15NativeImportRoundTrip(t, "Actions-4.0.1-observed-descriptor", root, plist.XMLFormat, plist.BinaryFormat)
	if len(observations) != 8 {
		t.Fatalf("expected all eight format/fidelity/dialect imports, got %d", len(observations))
	}
	for _, observation := range observations {
		if err := p15ActionsAppExactError(expected, observation.compiled); err != nil {
			t.Fatalf("%s/%s: %v", observation.fidelity, observation.mode, err)
		}
		if observation.fidelity == "hybrid" {
			if observation.plan == nil || len(observation.plan.Decisions) != 3 {
				t.Fatal("hybrid import omitted the complete three-action plan")
			}
			var decision = observation.plan.Decisions[1]
			if decision.Classification != "EXACT_NATIVE_FALLBACK" || decision.ReasonCode != "THIRD_PARTY_NO_CANONICAL_DEF" {
				t.Fatalf("an unregistered provider was advertised as a named callable: %+v", decision)
			}
		}
	}

	// Every corruption starts from the same actual successful imported output.
	// The healthy full-tree oracle above must pass before any negative can count.
	var baseline = observations[0].compiled
	var faults = []string{
		"empty_descriptor", "missing_descriptor", "wrong_team", "wrong_bundle", "wrong_intent", "wrong_name",
		"installation_false", "installation_string", "installation_missing", "outer_identifier",
		"producer_uuid", "consumer_uuid", "neighbor_changed", "extra_action",
	}
	for _, fault := range faults {
		t.Run("reject_"+fault, func(t *testing.T) {
			var actual = p15ActionsAppClone(baseline).(map[string]any)
			var actions = actual["WFWorkflowActions"].([]any)
			var params = actions[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
			var descriptor = params["AppIntentDescriptor"].(map[string]any)
			switch fault {
			case "empty_descriptor":
				params["AppIntentDescriptor"] = ""
			case "missing_descriptor":
				delete(params, "AppIntentDescriptor")
			case "wrong_team":
				descriptor["TeamIdentifier"] = appleLegacyTeamIdentifier
			case "wrong_bundle":
				descriptor["BundleIdentifier"] = "com.example.wrong-provider"
			case "wrong_intent":
				descriptor["AppIntentIdentifier"] = "UnsupportedIntent"
			case "wrong_name":
				descriptor["Name"] = "Other provider"
			case "installation_false":
				descriptor["ActionRequiresAppInstallation"] = false
			case "installation_string":
				descriptor["ActionRequiresAppInstallation"] = "true"
			case "installation_missing":
				delete(descriptor, "ActionRequiresAppInstallation")
			case "outer_identifier":
				actions[1].(map[string]any)["WFWorkflowActionIdentifier"] = "com.example.wrong-provider.IsLocationServicesEnabledIntent"
			case "producer_uuid":
				params["UUID"] = "40100000-0000-4000-8000-000000000002"
			case "consumer_uuid":
				actions[2].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFVariable"].(map[string]any)["Value"].(map[string]any)["OutputUUID"] = "40100000-0000-4000-8000-000000000002"
			case "neighbor_changed":
				actions[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFCommentActionText"] = "changed"
			case "extra_action":
				actual["WFWorkflowActions"] = append(actions, p15ActionsAppClone(actions[1]))
			}
			if p15ActionsAppExactError(expected, actual) == nil {
				t.Fatalf("exact descriptor/native oracle accepted %s", fault)
			}
			if err := p15ActionsAppExactError(expected, baseline); err != nil {
				t.Fatalf("negative mutated the healthy original: %v", err)
			}
		})
	}
	if len(faults) != 14 {
		t.Fatal("fixed negative inventory changed")
	}
}
