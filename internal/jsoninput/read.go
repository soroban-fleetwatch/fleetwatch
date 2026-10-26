package jsoninput

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

const MaxBytes = 25 << 20

// MaxObjectMembers bounds the duplicate-key set for each JSON object.
const MaxObjectMembers = 10000

func File(path string, target any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return Decode(file, target)
}

// Read the limit plus one byte so the size bound also applies to pipes and
// other non-seekable inputs. Never decode a silently truncated document.
func Decode(r io.Reader, target any) error {
	return decode(r, target, false)
}

// DecodeExtensible preserves the same byte, UTF-8, nesting and duplicate-member
// guards as Decode while allowing additive provider fields. Operator-controlled
// inputs must continue to use Decode, which rejects unknown fields.
func DecodeExtensible(r io.Reader, target any) error {
	return decode(r, target, true)
}

func decode(r io.Reader, target any, extensible bool) error {
	data, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return fmt.Errorf("read JSON: %w", err)
	}
	if len(data) > MaxBytes {
		return errors.New("JSON input exceeds 25 MiB limit")
	}
	if !utf8.Valid(data) {
		return errors.New("JSON input must be UTF-8")
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
	if !extensible {
		dec.DisallowUnknownFields()
	}
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
			folded := strings.ToUpper(strings.ToLower(name))
			if seen[folded] {
				return errors.New("duplicate JSON object member")
			}
			if len(seen) >= MaxObjectMembers {
				return errors.New("JSON object exceeds 10000 members")
			}
			seen[folded] = true
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
