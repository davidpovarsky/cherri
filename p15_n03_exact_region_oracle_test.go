package main

import (
	"fmt"
	"strings"
)

// A region is the complete native exact-Text value producer. End is inclusive;
// Next is the first action after it. Left and Right are observed operands, not
// independently established source intent: each consumer must bind them and
// Operator to its authored fixture before accepting the result.
type p15N03ExactNativeRegion struct {
	Start, End, Next, InnerStart, InnerEnd           int
	LeftSnapshot, RightSnapshot, Quote, Match, Count int
	TrueValue, Else, FalseValue                      int
	Operator                                         uint64
	Left, Right                                      any
	Result                                           map[string]any
	ActionIndexes                                    map[int]bool
}

func p15N03ExactSnapshotOperand(payload any) (any, error) {
	if text, okay := payload.(string); okay {
		if !strings.HasPrefix(text, "~") {
			return nil, fmt.Errorf("exact Text literal snapshot lost its sentinel")
		}
		return strings.TrimPrefix(text, "~"), nil
	}
	outer, okay := payload.(map[string]any)
	if !okay || len(outer) != 2 || outer["WFSerializationType"] != "WFTextTokenString" {
		return nil, fmt.Errorf("exact Text snapshot lost its native transport")
	}
	body, okay := outer["Value"].(map[string]any)
	if !okay || len(body) != 2 || body["string"] != "~\uFFFC" {
		return nil, fmt.Errorf("exact Text snapshot changed its sole operand")
	}
	attachments, okay := body["attachmentsByRange"].(map[string]any)
	if !okay || len(attachments) != 1 {
		return nil, fmt.Errorf("exact Text snapshot attachment inventory differs")
	}
	ref, okay := attachments["{1, 1}"].(map[string]any)
	if !okay || ref["Type"] == nil {
		return nil, fmt.Errorf("exact Text snapshot has no native operand reference")
	}
	return ref, nil
}

// This adapter reuses the existing primitive and complete cursor oracles. It
// adds the strict native-plist control kinds and canonical/case-folded group
// identity required by the calling native graph oracles; it is not a second
// implementation of exact Text comparison.
func p15N03ExactNativeRegions(actions []map[string]any, expected int) ([]p15N03ExactNativeRegion, error) {
	if err := p15N03ExactPrimitiveError(actions, expected); err != nil {
		return nil, err
	}
	var regions []p15N03ExactNativeRegion
	covered := map[int]bool{}
	groups := map[string]bool{}
	for match, action := range actions {
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.text.match" {
			continue
		}
		start, end := match-3, match+6
		if start < 0 || end >= len(actions) {
			return nil, fmt.Errorf("incomplete exact Text region around match %d", match)
		}
		left, err := p15N03ExactSnapshotOperand(p15N03ExactParams(actions[start])["WFTextActionText"])
		if err != nil {
			return nil, err
		}
		right, err := p15N03ExactSnapshotOperand(p15N03ExactParams(actions[start+1])["WFTextActionText"])
		if err != nil {
			return nil, err
		}
		operator, okay := p15N03ExactParams(actions[start+5])["WFCondition"].(uint64)
		if !okay || (operator != 4 && operator != 5) {
			return nil, fmt.Errorf("exact Text operator is not native equality/inequality")
		}
		name := ""
		if value, present := p15N03ExactParams(actions[end])["CustomOutputName"]; present {
			name, okay = value.(string)
			if !okay || name == "" {
				return nil, fmt.Errorf("exact Text result has an invalid binding name")
			}
		}
		cursor := p15N03Cursor{actions: p15DiagnosticGraph(actions), index: start, groups: map[string]bool{}}
		result := cursor.exactValueComparison(name, left, float64(operator), right)
		if cursor.err != nil {
			return nil, cursor.err
		}
		if cursor.index != end+1 {
			return nil, fmt.Errorf("exact Text region did not consume its complete producer")
		}
		region := p15N03ExactNativeRegion{
			Start: start, End: end, Next: end + 1, InnerStart: start + 5, InnerEnd: end,
			LeftSnapshot: start, RightSnapshot: start + 1, Quote: start + 2, Match: match, Count: start + 4,
			TrueValue: start + 6, Else: start + 7, FalseValue: start + 8,
			Operator: operator, Left: left, Right: right, Result: result, ActionIndexes: map[int]bool{},
		}
		for index := start; index <= end; index++ {
			if covered[index] || len(actions[index]) != 2 {
				return nil, fmt.Errorf("overlapping or extended exact Text action at %d", index)
			}
			parameters := p15N03ExactParams(actions[index])
			if value, present := parameters["UUID"]; present {
				uuid, okay := value.(string)
				if !okay || !p15SCCanonicalUUID(uuid) {
					return nil, fmt.Errorf("invalid exact Text producer identity at %d", index)
				}
			}
			if actions[index]["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" {
				mode, okay := parameters["WFControlFlowMode"].(uint64)
				group, named := parameters["GroupingIdentifier"].(string)
				if !okay || !named || !p15SCCanonicalUUID(group) {
					return nil, fmt.Errorf("invalid exact Text native control at %d", index)
				}
				if mode == 0 {
					key := strings.ToLower(group)
					if groups[key] {
						return nil, fmt.Errorf("reused exact Text condition group")
					}
					groups[key] = true
				}
			}
			covered[index], region.ActionIndexes[index] = true, true
		}
		regions = append(regions, region)
	}
	// Reserved helper names outside a validated region cannot be used to
	// smuggle an orphan snapshot/quote/count into a consumer's helper inventory.
	for index, action := range actions {
		name, _ := p15N03ExactParams(action)["CustomOutputName"].(string)
		switch name {
		case "_n03_exact_left", "_n03_exact_right", "_n03_exact_quote", "_n03_exact_match", "_n03_exact_count":
			if !covered[index] {
				return nil, fmt.Errorf("orphan exact Text helper at %d", index)
			}
		}
	}
	return regions, nil
}

// Sequential complete-graph consumers validate one region when their authored
// cursor reaches it. Existing checks on the surrounding graph keep their own
// causal failure sites; the full consumer must still reject unconsumed actions
// and validate every external operand producer before accepting the graph.
func p15N03ExactNativeRegionAt(actions []map[string]any, start int) (p15N03ExactNativeRegion, error) {
	if start < 0 || start > len(actions)-10 {
		return p15N03ExactNativeRegion{}, fmt.Errorf("missing exact Text region at %d", start)
	}
	regions, err := p15N03ExactNativeRegions(actions[start:start+10], 1)
	if err != nil {
		return p15N03ExactNativeRegion{}, err
	}
	region := regions[0]
	region.Start += start
	region.End += start
	region.Next += start
	region.InnerStart += start
	region.InnerEnd += start
	region.LeftSnapshot += start
	region.RightSnapshot += start
	region.Quote += start
	region.Match += start
	region.Count += start
	region.TrueValue += start
	region.Else += start
	region.FalseValue += start
	region.ActionIndexes = map[int]bool{}
	for index := region.Start; index <= region.End; index++ {
		region.ActionIndexes[index] = true
	}
	return region, nil
}

// An authored plain If over an exact comparison must consume that comparison's
// actual Boolean result and compare it to real one. This validates only the
// start action; its body and matching end remain the caller's responsibility.
func p15N03ExactResultGuard(actions []map[string]any, region p15N03ExactNativeRegion, index int) (string, error) {
	if index != region.Next || index < 0 || index >= len(actions) || len(actions[index]) != 2 ||
		region.InnerStart < 0 || region.InnerStart >= len(actions) {
		return "", fmt.Errorf("exact Text result is not followed by its authored guard")
	}
	parameters := p15N03ExactParams(actions[index])
	mode, modeOK := parameters["WFControlFlowMode"].(uint64)
	code, codeOK := parameters["WFCondition"].(uint64)
	group, groupOK := parameters["GroupingIdentifier"].(string)
	innerGroup, innerOK := p15N03ExactParams(actions[region.InnerStart])["GroupingIdentifier"].(string)
	if !modeOK || mode != 0 || !codeOK || code != 4 || !groupOK || !p15SCCanonicalUUID(group) ||
		!innerOK || strings.EqualFold(group, innerGroup) {
		return "", fmt.Errorf("invalid authored exact Text Boolean guard")
	}
	cursor := p15N03Cursor{actions: p15DiagnosticGraph(actions), index: index, groups: map[string]bool{}}
	observed := cursor.start(region.Result, 4, "WFNumberValue", float64(1))
	if cursor.err != nil {
		return "", cursor.err
	}
	return observed, nil
}
