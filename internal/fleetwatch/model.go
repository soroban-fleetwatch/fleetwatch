// Package fleetwatch compares independently captured Soroban contract fleet
// snapshots. It does not currently collect or authenticate network data.
package fleetwatch

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const SchemaVersion = 1

// Local interchange limits, not claims about protocol-level limits.
const (
	MaxRecords         = 10000
	MaxIdentifierBytes = 256
	MaxTagBytes        = 1024
	MaxTicketBytes     = 1024
)

// v1 retains opaque synthetic identifiers. Reject separators/control bytes
// that could make composite keys ambiguous or inject terminal output.
func validIdentifier(s string) bool {
	if len(s) == 0 || len(s) > MaxIdentifierBytes || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if unicode.IsSpace(r) || unicode.IsControl(r) || r == '/' {
			return false
		}
	}
	return true
}

func validText(s string, limit int) bool {
	if strings.TrimSpace(s) == "" || len(s) > limit || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// Snapshot is an explicitly captured state at a single Stellar ledger. All
// members of a fleet and their external executable reference entries should
// be included. A missing entry cannot be interpreted as on-chain deletion.
type Snapshot struct {
	SchemaVersion int         `json:"schema_version"`
	Network       string      `json:"network"`
	Ledger        uint32      `json:"ledger"`
	Contracts     []Contract  `json:"contracts"`
	References    []Reference `json:"references"`
}

type Contract struct {
	ID         string     `json:"id"`
	Executable Executable `json:"executable"`
}

type Executable struct {
	Kind     string `json:"kind"` // wasm, external_ref, stellar_asset
	WasmHash string `json:"wasm_hash,omitempty"`
	Owner    string `json:"owner,omitempty"`
	TagHex   string `json:"tag_hex,omitempty"`
}

type Reference struct {
	Owner    string `json:"owner"`
	TagHex   string `json:"tag_hex"` // CAP-85 tag's original bytes in hex
	WasmHash string `json:"wasm_hash"`
}

type Approval struct {
	Owner    string `json:"owner"`
	TagHex   string `json:"tag_hex"`
	FromHash string `json:"from_hash"`
	ToHash   string `json:"to_hash"`
	Ticket   string `json:"ticket"`
}

type Policy struct {
	SchemaVersion   int        `json:"schema_version"`
	ApprovedChanges []Approval `json:"approved_changes"`
}

type Finding struct {
	Code              string   `json:"code"`
	Severity          string   `json:"severity"` // critical, high, warning, info
	Message           string   `json:"message"`
	Owner             string   `json:"owner,omitempty"`
	TagHex            string   `json:"tag_hex,omitempty"`
	ContractID        string   `json:"contract_id,omitempty"`
	OldHash           string   `json:"old_hash,omitempty"`
	NewHash           string   `json:"new_hash,omitempty"`
	AffectedContracts []string `json:"affected_contracts,omitempty"`
	ApprovalTicket    string   `json:"approval_ticket,omitempty"`
	Authorization     string   `json:"authorization"`
}

type Report struct {
	SchemaVersion int       `json:"schema_version"`
	Network       string    `json:"network"`
	BeforeLedger  uint32    `json:"before_ledger"`
	AfterLedger   uint32    `json:"after_ledger"`
	Findings      []Finding `json:"findings"`
	// Data source must be verified separately; this is not a cryptographic attestation.
	EvidenceDisclaimer string `json:"evidence_disclaimer"`
	Coverage           string `json:"coverage"`
	EvaluationStatus   string `json:"evaluation_status"`
}

func refKey(owner, tag string) string { return owner + "/" + strings.ToLower(tag) }

func validHash(h string) bool {
	if len(h) != 64 {
		return false
	}
	_, err := hex.DecodeString(h)
	return err == nil
}

func validTag(t string) bool {
	return len(t) > 0 && len(t) <= MaxTagBytes*2 && len(t)%2 == 0 && isHex(t)
}
func isHex(s string) bool { _, err := hex.DecodeString(s); return err == nil }

func validateExecutable(ex Executable) error {
	switch ex.Kind {
	case "wasm":
		if !validHash(ex.WasmHash) || ex.Owner != "" || ex.TagHex != "" {
			return errors.New("wasm requires a 32-byte wasm_hash and no owner/tag")
		}
	case "external_ref":
		if !validIdentifier(ex.Owner) || !validTag(ex.TagHex) || ex.WasmHash != "" {
			return errors.New("external_ref requires owner and byte-exact tag_hex, without wasm_hash")
		}
	case "stellar_asset":
		if ex.WasmHash != "" || ex.Owner != "" || ex.TagHex != "" {
			return errors.New("stellar_asset must not have additional fields")
		}
	default:
		return fmt.Errorf("unsupported executable kind %q", ex.Kind)
	}
	return nil
}

func (s Snapshot) Validate() error {
	if s.SchemaVersion != SchemaVersion {
		return fmt.Errorf("snapshot schema_version must be %d", SchemaVersion)
	}
	if !validText(s.Network, MaxIdentifierBytes) || s.Ledger == 0 {
		return errors.New("network and positive ledger required")
	}
	if len(s.Contracts) > MaxRecords || len(s.References) > MaxRecords {
		return errors.New("snapshot exceeds record limit")
	}
	seenContracts := make(map[string]bool)
	for i, c := range s.Contracts {
		if !validIdentifier(c.ID) {
			return fmt.Errorf("contracts[%d]: invalid opaque id", i)
		}
		if seenContracts[c.ID] {
			return fmt.Errorf("duplicate contract id: %s", c.ID)
		}
		seenContracts[c.ID] = true
		if err := validateExecutable(c.Executable); err != nil {
			return fmt.Errorf("contracts[%d]: %w", i, err)
		}
	}
	seenRefs := make(map[string]bool)
	for i, r := range s.References {
		if !validIdentifier(r.Owner) || !validTag(r.TagHex) || !validHash(r.WasmHash) {
			return fmt.Errorf("references[%d]: invalid owner, tag_hex or wasm_hash", i)
		}
		key := refKey(r.Owner, r.TagHex)
		if seenRefs[key] {
			return fmt.Errorf("duplicate reference: %s", key)
		}
		seenRefs[key] = true
	}
	return nil
}

func (p Policy) Validate() error {
	if p.SchemaVersion != SchemaVersion {
		return fmt.Errorf("policy schema_version must be %d", SchemaVersion)
	}
	if len(p.ApprovedChanges) > MaxRecords {
		return errors.New("policy exceeds record limit")
	}
	seen := make(map[string]bool)
	for i, a := range p.ApprovedChanges {
		if !validIdentifier(a.Owner) || !validTag(a.TagHex) || !validHash(a.FromHash) || !validHash(a.ToHash) || !validText(a.Ticket, MaxTicketBytes) {
			return fmt.Errorf("approved_changes[%d]: owner, tag_hex, 32-byte from/to hashes and ticket required", i)
		}
		key := fmt.Sprintf("%s/%s/%s/%s", a.Owner, strings.ToLower(a.TagHex), strings.ToLower(a.FromHash), strings.ToLower(a.ToHash))
		if seen[key] {
			return fmt.Errorf("duplicate approval: %s", key)
		}
		seen[key] = true
	}
	return nil
}
