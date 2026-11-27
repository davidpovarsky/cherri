package main

// Runtime companions preserve the original FN04 programs and complete native
// graph assertions. Names are chosen through the compiler filename API; no
// serialized workflow is edited. The no-functions companion binds the outputs
// of the same two input-read actions so their actual values can be observed.
import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"testing"
)

const p15NativeCallerPacketSchema = "cherri-native-caller-input-packet/1"

type p15NativeCallerPacket struct {
	Schema                 string `json:"schema"`
	CaseID                 string `json:"caseId"`
	PacketToken            string `json:"packetToken"`
	WorkflowName           string `json:"workflowName"`
	CompileFilename        string `json:"compileFilename"`
	CanonicalTest          string `json:"canonicalTest"`
	CanonicalSourceBase64  string `json:"canonicalSourceBase64"`
	CanonicalSourceSHA256  string `json:"canonicalSourceSha256"`
	SourceBase64           string `json:"sourceBase64"`
	SourceSHA256           string `json:"sourceSha256"`
	SourceBytes            int    `json:"sourceBytes"`
	UnsignedBase64         string `json:"unsignedBase64"`
	UnsignedSHA256         string `json:"unsignedSha256"`
	UnsignedBytes          int    `json:"unsignedBytes"`
	ActionCount            int    `json:"actionCount"`
	OriginalGraphOracle    string `json:"originalGraphOracle"`
	CompanionKind          string `json:"companionKind"`
	NativeExecuted         bool   `json:"nativeExecuted"`
	FormalAcceptanceGranted bool  `json:"formalAcceptanceGranted"`
}

func p15NativeCallerName(caseID, token string) (string, error) {
	if !regexp.MustCompile("^[0-9a-f]{32}$").MatchString(token) {
		return "", fmt.Errorf("native packet token must be exactly 32 lowercase hexadecimal characters")
	}
	prefix := ""
	switch caseID {
	case "nested_false":
		prefix = "CherriFN04Direct_"
	case "nested_true":
		prefix = "CherriFN04Nested_"
	case "without_functions":
		prefix = "CherriFN04Plain_"
	default:
		return "", fmt.Errorf("unenrolled FN04 native packet case")
	}
	return prefix + token, nil
}

func p15NativeCallerPlainSource() string {
	return "let first=getVariable(system.input)\nlet second=getVariable(ShortcutInput)\n" +
		"setClipboard(f\"{first}|{second}\")"
}

// Every original input-read action is still modeled. Only the two UUID edges
// and one clipboard consumer are added; the original main Nothing is retained.
func p15NativeCallerPlainGraph(actions []map[string]any) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	input := p15FunctionAttachment(map[string]any{"Type": "ExtensionInput"})
	first := g.value("getvariable", map[string]any{"WFVariable": input})
	second := g.value("getvariable", map[string]any{"WFVariable": input})
	g.emit("setclipboard", map[string]any{"WFInput": p15CallerInputText("\ufffc|\ufffc", p15FunctionEdge(first), p15FunctionEdge(second))})
	g.emit("nothing", nil)
	return g.compare()
}

func p15NativeCallerEmit(t *testing.T, caseID, token, name, canonical, source, kind, oracle string, raw []byte, actions []map[string]any) {
	t.Helper()
	testName := "TestP15FN04CallerInputCompleteNativeGraphs/" + caseID
	if caseID == "without_functions" {
		testName = "TestP15FN04CallerInputWithoutFunctions"
	}
	value := p15NativeCallerPacket{
		Schema: p15NativeCallerPacketSchema, CaseID: caseID, PacketToken: token,
		WorkflowName: name, CompileFilename: name + ".cherri", CanonicalTest: testName,
		CanonicalSourceBase64: base64.StdEncoding.EncodeToString([]byte(canonical)),
		CanonicalSourceSHA256: p15NativeContractSHA([]byte(canonical)),
		SourceBase64: base64.StdEncoding.EncodeToString([]byte(source)),
		SourceSHA256: p15NativeContractSHA([]byte(source)), SourceBytes: len([]byte(source)),
		UnsignedBase64: base64.StdEncoding.EncodeToString(raw),
		UnsignedSHA256: p15NativeContractSHA(raw), UnsignedBytes: len(raw), ActionCount: len(actions),
		OriginalGraphOracle: oracle, CompanionKind: kind,
		NativeExecuted: false, FormalAcceptanceGranted: false,
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("P15_NATIVE_CALLER_PACKET sha256=%s bytes=%d base64=%s",
		p15NativeContractSHA(encoded), len(encoded), base64.StdEncoding.EncodeToString(encoded))
}

func TestP15NativeCallerInputPackets(t *testing.T) {
	token := os.Getenv("CHERRI_NATIVE_PACKET_TOKEN")
	if token == "" {
		// Ordinary unit tests use a fixed non-native fixture identity. An actual
		// import owner must provide and bind its fresh packet token separately.
		token = "00000000000000000000000000000000"
	}
	for _, invalid := range []struct{ name, token string }{
		{"", token}, {"nested_false_extra", token}, {"nested_true", ""},
		{"nested_false", "0000000000000000000000000000000G"},
	} {
		if _, err := p15NativeCallerName(invalid.name, invalid.token); err == nil {
			t.Fatal("unbound native case/name was accepted")
		}
	}
	for _, caseID := range []string{"nested_false", "nested_true", "without_functions"} {
		t.Run(caseID, func(t *testing.T) {
			name, err := p15NativeCallerName(caseID, token)
			if err != nil {
				t.Fatal(err)
			}
			nested := caseID == "nested_true"
			canonical := p15CallerInputSource(nested)
			source, kind, oracle := canonical, "ORIGINAL_SOURCE_COMPILER_NAME", "p15CallerInputGraphForWorkflow"
			if caseID == "without_functions" {
				canonical = "getVariable(system.input)\ngetVariable(ShortcutInput)"
				source, kind, oracle = p15NativeCallerPlainSource(), "CAPTURE_ORIGINAL_INPUT_READ_RESULTS", "p15NativeCallerPlainGraph"
				p15FunctionTypedProgram(t, source, map[string]string{
					"getVariable(system.input)": "AnyContent", "getVariable(ShortcutInput)": "AnyContent",
				})
			} else {
				callee := "inspect"
				if nested {
					callee = "outer"
				}
				p15FunctionTypedProgram(t, source, map[string]string{callee + `("VALUE")`: "Text"})
			}
			raw, _, actions := p15NativeContractCompile(t, name+".cherri", source)
			if caseID == "without_functions" {
				if err := p15NativeCallerPlainGraph(actions); err != nil {
					t.Fatal(err)
				}
				bad := p15C01Clone(t, actions)
				p15C01Params(bad[0])["WFVariable"] = p15FunctionText("WRONG_INPUT")
				if p15NativeCallerPlainGraph(bad) == nil {
					t.Fatal("replacement of the original caller-input read was accepted")
				}
			} else {
				if _, err := p15CallerInputGraphForWorkflow(actions, nested, name); err != nil {
					t.Fatal(err)
				}
				bad := p15C01Clone(t, actions)
				changed := false
				for _, action := range bad {
					if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.runworkflow" {
						p15C01Params(action)["WFWorkflowName"] = name + "_wrong"
						changed = true
						break
					}
				}
				if !changed {
					t.Fatal("the original function-call action was not present")
				}
				if _, err := p15CallerInputGraphForWorkflow(bad, nested, name); err == nil {
					t.Fatal("a call to a different native workflow was accepted")
				}
			}
			p15NativeCallerEmit(t, caseID, token, name, canonical, source, kind, oracle, raw, actions)
		})
	}
}
