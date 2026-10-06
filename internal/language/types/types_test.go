package types

import (
	"testing"
)

func TestTypeAssignability(t *testing.T) {
	// Text to Text
	if !Text.AssignableTo(Text) {
		t.Errorf("Text should be assignable to Text")
	}
	// Text not to Number
	if Text.AssignableTo(Number) {
		t.Errorf("Text should not be assignable to Number")
	}
	// Anything to Unknown / AnyContent
	if !Text.AssignableTo(Unknown) || !Text.AssignableTo(AnyContent) {
		t.Errorf("Text should be assignable to Unknown and AnyContent")
	}
	// T to T?
	optText := NewOptional(Text)
	if !Text.AssignableTo(optText) {
		t.Errorf("Text should be assignable to Text?")
	}
	// T? to T is false
	if optText.AssignableTo(Text) {
		t.Errorf("Text? should not be assignable to Text")
	}

	// List<Text> to List<Text>
	listText := NewList(Text)
	if !listText.AssignableTo(listText) {
		t.Errorf("List<Text> should be assignable to List<Text>")
	}
	listNum := NewList(Number)
	if listText.AssignableTo(listNum) {
		t.Errorf("List<Text> should not be assignable to List<Number>")
	}

	// Union
	union := NewUnion(Text, Number)
	if !Text.AssignableTo(union) || !Number.AssignableTo(union) {
		t.Errorf("Text and Number should be assignable to Text | Number")
	}
	if Bool.AssignableTo(union) {
		t.Errorf("Bool should not be assignable to Text | Number")
	}

	// Record width subtyping: {a: Text, b: Number} is assignable to {a: Text}
	rec1 := NewRecord([]RecordField{
		{Name: "a", Type: Text},
		{Name: "b", Type: Number},
	})
	rec2 := NewRecord([]RecordField{
		{Name: "a", Type: Text},
	})
	if !rec1.AssignableTo(rec2) {
		t.Errorf("Record {a, b} should be assignable to {a}")
	}
	if rec2.AssignableTo(rec1) {
		t.Errorf("Record {a} should not be assignable to {a, b}")
	}
}
