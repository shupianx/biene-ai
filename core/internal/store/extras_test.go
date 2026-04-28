package store

import (
	"encoding/json"
	"strings"
	"testing"
)

type sample struct {
	Name string `json:"name"`
	Age  int    `json:"age,omitempty"`
}

func TestRoundTripPreservesUnknownFields(t *testing.T) {
	raw := []byte(`{"name":"alice","age":30,"avatar":"7","ui_pref":{"theme":"dark"}}`)

	var s sample
	var extras Extras
	if err := UnmarshalWithExtras(raw, &s, &extras); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.Name != "alice" || s.Age != 30 {
		t.Fatalf("typed fields wrong: %+v", s)
	}
	if _, ok := extras["avatar"]; !ok {
		t.Fatal("avatar should be in extras")
	}
	if _, ok := extras["ui_pref"]; !ok {
		t.Fatal("ui_pref should be in extras")
	}
	if _, ok := extras["name"]; ok {
		t.Fatal("typed field name leaked into extras")
	}

	// Modify a typed field and re-marshal; extras must survive.
	s.Age = 31
	out, err := MarshalWithExtras(s, extras)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var roundTripped map[string]json.RawMessage
	if err := json.Unmarshal(out, &roundTripped); err != nil {
		t.Fatalf("re-decode: %v", err)
	}
	if string(roundTripped["age"]) != "31" {
		t.Fatalf("typed field not updated, got %s", roundTripped["age"])
	}
	if string(roundTripped["avatar"]) != `"7"` {
		t.Fatalf("avatar lost, got %q", roundTripped["avatar"])
	}
	if string(roundTripped["ui_pref"]) != `{"theme":"dark"}` {
		t.Fatalf("ui_pref lost, got %s", roundTripped["ui_pref"])
	}
}

func TestTypedFieldsWinOverExtras(t *testing.T) {
	// Conflict: extras shouldn't be able to override a typed field's value.
	s := sample{Name: "real", Age: 22}
	extras := Extras{
		"name": json.RawMessage(`"impostor"`),
		"x":    json.RawMessage(`"keep"`),
	}
	out, err := MarshalWithExtras(s, extras)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), `"name":"real"`) {
		t.Fatalf("typed field overridden by extras: %s", out)
	}
	if !strings.Contains(string(out), `"x":"keep"`) {
		t.Fatalf("non-conflicting extras lost: %s", out)
	}
}

func TestNilExtrasMarshalsLikePlain(t *testing.T) {
	s := sample{Name: "x", Age: 1}
	withExtras, err := MarshalWithExtras(s, nil)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	plain, _ := json.Marshal(s)
	if string(withExtras) != string(plain) {
		t.Fatalf("nil extras should match plain marshal\n  got:  %s\n  want: %s", withExtras, plain)
	}
}

func TestMarshalIsByteStable(t *testing.T) {
	// Same struct + same extras must produce identical bytes across calls
	// regardless of map iteration order. Otherwise meta.json churns on
	// every save just because Go shuffled the map.
	s := sample{Name: "x", Age: 1}
	extras := Extras{
		"z": json.RawMessage(`1`),
		"a": json.RawMessage(`2`),
		"m": json.RawMessage(`3`),
	}
	first, err := MarshalWithExtras(s, extras)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for i := 0; i < 20; i++ {
		next, err := MarshalWithExtras(s, extras)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if string(first) != string(next) {
			t.Fatalf("marshal is not byte-stable across calls\n  first: %s\n  next:  %s", first, next)
		}
	}
}

func TestEmptyExtrasNotSerialized(t *testing.T) {
	raw := []byte(`{"name":"a","age":1}`)
	var s sample
	var extras Extras
	if err := UnmarshalWithExtras(raw, &s, &extras); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if extras != nil {
		t.Fatalf("expected nil extras when input has no extra keys, got %+v", extras)
	}
}

func TestUnmarshalRejectsNonObject(t *testing.T) {
	var s sample
	var extras Extras
	if err := UnmarshalWithExtras([]byte(`["not","an","object"]`), &s, &extras); err == nil {
		t.Fatal("expected error on non-object input")
	}
}
