package fleetwatch

import (
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	"io"
)

const maxInputBytes = jsoninput.MaxBytes

func readStrict(path string, target any) error   { return jsoninput.File(path, target) }
func decodeStrict(r io.Reader, target any) error { return jsoninput.Decode(r, target) }

func LoadSnapshot(path string) (Snapshot, error) {
	var s Snapshot
	if err := readStrict(path, &s); err != nil {
		return s, err
	}
	return s, s.Validate()
}

func LoadPolicy(path string) (Policy, error) {
	if path == "" {
		return Policy{SchemaVersion: SchemaVersion}, nil
	}
	var p Policy
	if err := readStrict(path, &p); err != nil {
		return p, err
	}
	return p, p.Validate()
}
