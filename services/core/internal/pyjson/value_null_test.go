package pyjson

import "testing"

// The doc on Value says "A nil Value is treated as Null by the encoders". That
// is true when WRITING and false when READING, and the difference is a trap:
// `Null` is a struct, so an interface holding it is not nil. Every `== nil`
// guard on a value that came out of a decoded document therefore passes a
// JSON `null` straight through.
//
// This is not a rare shape. When no model key is configured the brain answers
// `{"card": null}`, so the guard that is supposed to stop there is exactly the
// one that does not.
func TestAJSONNullIsNotAGoNil(t *testing.T) {
	var fromDocument Value = Null{}
	if fromDocument == nil {
		t.Fatal("Null{} compared equal to nil; this test has lost its point")
	}
	if !IsNull(fromDocument) {
		t.Fatal("IsNull did not recognise Null{}")
	}
	var missing Value
	if !IsNull(missing) {
		t.Fatal("IsNull must also answer true for an absent value")
	}
	for _, present := range []Value{Bool(false), NewInt(0), Float(0), String(""), List{}, NewOrderedMap()} {
		if IsNull(present) {
			t.Fatalf("IsNull said %T was null; a present empty value is not null", present)
		}
	}
}

// The shape the decoder actually produces, rather than one written by hand.
func TestADecodedNullReadsBackAsNull(t *testing.T) {
	value, err := Loads([]byte(`{"card": null}`))
	if err != nil {
		t.Fatal(err)
	}
	object, ok := value.(*OrderedMap)
	if !ok {
		t.Fatalf("want an object, got %T", value)
	}
	card, present := object.Get("card")
	if !present {
		t.Fatal("the key is in the document; Get should report it present")
	}
	if card == nil {
		t.Fatal("decoded null came back as a Go nil; the trap would not exist")
	}
	if !IsNull(card) {
		t.Fatalf("IsNull did not recognise the decoded null (%T)", card)
	}
}
