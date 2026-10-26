package fleetwatch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

const maxInputBytes = 25 << 20

func readStrict(path string, target any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return decodeStrict(file, target)
}

// Read the limit plus one byte so the size bound also applies to pipes and
// other non-seekable inputs. Never decode a silently truncated document.
func decodeStrict(r io.Reader, target any) error {
	data, err := io.ReadAll(io.LimitReader(r, maxInputBytes+1))
	if err != nil {
		return fmt.Errorf("read JSON: %w", err)
	}
	if len(data) > maxInputBytes {
		return errors.New("JSON input exceeds 25 MiB limit")
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return errors.New("JSON document must be an object, not null")
	}
	keys := json.NewDecoder(bytes.NewReader(data))
	keys.UseNumber()
	if err := uniqueObjectKeys(keys, 0); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			return errors.New("input contains extra JSON values")
		}
		return fmt.Errorf("decode trailing data: %w", err)
	}
	return nil
}

// Reject ambiguous duplicate members, including nested executable/policy
// objects. A separate nesting bound limits work on adversarial documents.
func uniqueObjectKeys(dec *json.Decoder, depth int) error {
	if depth > 64 {
		return errors.New("JSON nesting exceeds 64 levels")
	}
	token, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]bool)
		for dec.More() {
			key, err := dec.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok {
				return errors.New("invalid JSON object key")
			}
			if seen[name] {
				return errors.New("duplicate JSON object member")
			}
			seen[name] = true
			if err := uniqueObjectKeys(dec, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for dec.More() {
			if err := uniqueObjectKeys(dec, depth+1); err != nil {
				return err
			}
		}
	default:
		return errors.New("unexpected JSON delimiter")
	}
	_, err = dec.Token()
	return err
}

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
