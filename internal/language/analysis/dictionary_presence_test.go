package analysis

import (
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func TestDictionaryPresenceFactsAreExactAndIsolated(t *testing.T) {
	analyzer := NewAnalyzer(schema.DefaultRegistry())
	for _, mutable := range []bool{false, true, false} {
		binding := "let"
		if mutable {
			binding = "var"
		}
		program := registryProgram(t, binding+` source={"key":""};let checked=require(source,type:"dictionary");let selected:Text?=checked["key"]`)
		before := syntax.Format(program)
		analyzer.Analyze(program)
		if syntax.Format(program) != before {
			t.Fatal("presence analysis mutated source")
		}
		selected := program.Statements[2].(*syntax.BindingStmt).Value
		facts := analyzer.KnownPresentDictionaryIndexes()
		if mutable {
			if len(facts) != 0 || len(analyzer.Diagnostics()) != 1 || analyzer.Diagnostics()[0].Code != CodeTargetValueShape {
				t.Fatal("mutable source borrowed immutable proof", facts, analyzer.Diagnostics())
			}
		} else {
			if len(analyzer.Diagnostics()) != 0 || len(facts) != 1 || !facts[selected] {
				t.Fatal("literal identity lost or unrelated node promoted", facts, analyzer.Diagnostics())
			}
			delete(facts, selected)
			facts[program.Statements[0].(*syntax.BindingStmt).Value] = true
			fresh := analyzer.KnownPresentDictionaryIndexes()
			if len(fresh) != 1 || !fresh[selected] {
				t.Fatal("caller mutation changed analyzer-owned facts")
			}
		}
	}
}

func TestDictionaryPresenceHonorsLexicalShadowing(t *testing.T) {
	program := registryProgram(t, `let dictionary={"key":""}
let outer:Text?=dictionary["key"]
if true { var dictionary={"other":"changed"};let inner:Text?=dictionary["key"] }
let after:Text?=dictionary["key"]`)
	a := NewAnalyzer(schema.DefaultRegistry())
	a.Analyze(program)
	facts := a.KnownPresentDictionaryIndexes()
	outer := program.Statements[1].(*syntax.BindingStmt).Value
	after := program.Statements[3].(*syntax.BindingStmt).Value
	if len(facts) != 2 || !facts[outer] || !facts[after] {
		t.Fatal("shadowed binding borrowed or removed outer origin", facts)
	}
	errors := 0
	for _, d := range a.Diagnostics() {
		if d.Severity == SeverityError {
			errors++
			if d.Code != CodeTargetValueShape {
				t.Fatal(d)
			}
		}
	}
	if errors != 1 {
		t.Fatal("inner mutable lookup was not independently rejected", a.Diagnostics())
	}
}
