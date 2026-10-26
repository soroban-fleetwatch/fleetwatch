package source

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
)

// decodeRPCResponse checks the complete document, including unknown extension
// fields and raw result objects, before interpreting a typed result. Providers
// can add fields but cannot shadow protocol members or choose ambiguous values.
func decodeRPCResponse(reader io.Reader, id uint64, target any) error {
	var envelope map[string]json.RawMessage
	if err := jsoninput.DecodeExtensible(reader, &envelope); err != nil {
		return errors.New("invalid RPC JSON response")
	}
	for key := range envelope {
		for _, canonical := range []string{"jsonrpc", "id", "result", "error"} {
			if key != canonical && strings.EqualFold(key, canonical) {
				return errors.New("noncanonical RPC envelope member")
			}
		}
	}
	var version string
	var responseID uint64
	if json.Unmarshal(envelope["jsonrpc"], &version) != nil || version != "2.0" || bytes.Equal(envelope["id"], []byte("null")) || json.Unmarshal(envelope["id"], &responseID) != nil || responseID != id {
		return errors.New("RPC error or mismatched response envelope")
	}
	if raw, found := envelope["error"]; found && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return errors.New("RPC error or mismatched response envelope")
	}
	result := bytes.TrimSpace(envelope["result"])
	if len(result) == 0 || result[0] != '{' {
		return errors.New("invalid RPC result")
	}
	if err := json.Unmarshal(result, target); err != nil {
		return errors.New("invalid RPC result")
	}
	return nil
}
