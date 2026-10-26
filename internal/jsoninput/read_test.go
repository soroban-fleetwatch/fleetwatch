package jsoninput

import (
	"bytes"
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
