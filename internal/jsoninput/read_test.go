package jsoninput

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestRejectsAmbiguousKeysAndInvalidUTF8(t *testing.T) {
	for _, raw := range [][]byte{[]byte(`{"schema_version":1,"Schema_Version":2}`), []byte(`{"key_id":1,"\u212Aey_id":2}`), []byte{'{', '"', 'x', '"', ':', '"', 255, '"', '}'}} {
		var out struct {
			SchemaVersion int    `json:"schema_version"`
			KeyID         int    `json:"key_id"`
			X             string `json:"x"`
		}
		if err := Decode(bytes.NewReader(raw), &out); err == nil {
			t.Fatalf("ambiguous JSON accepted: %q", raw)
		}
	}
}
func TestSharedDecoderRetainsStrictStructure(t *testing.T) {
	var out struct {
		Version int `json:"version"`
	}
	if err := Decode(strings.NewReader(`{"version":1}`), &out); err != nil || out.Version != 1 {
		t.Fatal(err)
	}
	for _, raw := range []string{`{"extra":1}`, `null`, `{"version":1} {}`, strings.Repeat("[", 66) + strings.Repeat("]", 66)} {
		if err := Decode(strings.NewReader(raw), &out); err == nil {
			t.Fatalf("invalid JSON accepted: %s", raw)
		}
	}
}

func TestExtensibleDecodePreservesStructuralGuards(t *testing.T) {
	var out struct {
		Known int `json:"known"`
	}
	if err := DecodeExtensible(strings.NewReader(`{"known":1,"provider_extension":{"enabled":true}}`), &out); err != nil || out.Known != 1 {
		t.Fatal(out, err)
	}
	if err := Decode(strings.NewReader(`{"known":1,"provider_extension":true}`), &out); err == nil {
		t.Fatal("strict operator input allowed unknown field")
	}
	for _, raw := range []string{`{"known":1,"Known":2}`, `{"known":1,"extension":{"a":1,"a":2}}`, `null`, `{"known":1}{}`, strings.Repeat("[", 66) + strings.Repeat("]", 66), "{\"known\":1,\"extra\":\"" + string([]byte{255}) + "\"}"} {
		if err := DecodeExtensible(strings.NewReader(raw), &out); err == nil {
			t.Fatalf("ambiguous extensible input accepted: %q", raw)
		}
	}
}
func TestObjectMemberBudget(t *testing.T) {
	var b strings.Builder
	b.WriteString("{")
	for i := 0; i < MaxObjectMembers; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "\"key%d\":0", i)
	}
	boundary := b.String() + "}"
	var out map[string]any
	if err := DecodeExtensible(strings.NewReader(boundary), &out); err != nil || len(out) != MaxObjectMembers {
		t.Fatal(len(out), err)
	}
	if err := DecodeExtensible(strings.NewReader(b.String()+`,"overflow":0}`), &out); err == nil {
		t.Fatal("oversized key set accepted")
	}
	if err := DecodeExtensible(strings.NewReader(`{"extension":`+b.String()+`,"overflow":0}}`), &out); err == nil {
		t.Fatal("oversized nested key set accepted")
	}
}
