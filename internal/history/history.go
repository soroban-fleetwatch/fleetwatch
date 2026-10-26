// Package history collects bounded, untrusted ledger-close metadata separately
// from replay state. It does not apply changes or authenticate consensus.
package history

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const MaxLedgers = 16
const MaxMetadataBytes = 4 << 20
const maxEvidenceBytes = 20 << 20

type RPC interface {
	Network(context.Context) (source.Network, error)
	Ledgers(context.Context, protocol.GetLedgersRequest) (protocol.GetLedgersResponse, error)
}

type Evidence struct {
	Sequence        uint32 `json:"sequence"`
	ProtocolVersion uint32 `json:"protocol_version"`
	CloseTime       int64  `json:"close_time"`
	Hash            string `json:"hash"`
	PreviousHash    string `json:"previous_hash"`
	MetadataVersion int32  `json:"metadata_version"`
	MetadataSHA256  string `json:"metadata_sha256"`
	HeaderXDR       string `json:"header_xdr"`
	MetadataXDR     string `json:"metadata_xdr"`
}

type Capture struct {
	SchemaVersion        int        `json:"schema_version"`
	Kind                 string     `json:"kind"`
	Source               string     `json:"source"`
	Network              string     `json:"network"`
	FromLedger           uint32     `json:"from_ledger"`
	ToLedger             uint32     `json:"to_ledger"`
	ProcessedThrough     uint32     `json:"processed_through"`
	SequenceCoverage     string     `json:"sequence_coverage"`
	SourceAuthentication string     `json:"source_authentication"`
	HistoryCompleteness  string     `json:"history_completeness"`
	MetadataCommitment   string     `json:"metadata_commitment"`
	StateChanges         string     `json:"state_changes"`
	FinalVerdict         string     `json:"final_verdict"`
	Ledgers              []Evidence `json:"ledgers"`
	Errors               []string   `json:"errors"`
}

func ValidateRequest(network string, from, to uint32) error {
	if strings.TrimSpace(network) == "" || len(network) > 256 || !utf8.ValidString(network) || from == 0 || to < from || uint64(to)-uint64(from) >= MaxLedgers {
		return errors.New("invalid network or history range (maximum 16 ledgers)")
	}
	for _, r := range network {
		if unicode.IsControl(r) {
			return errors.New("invalid network passphrase")
		}
	}
	return nil
}

// Fetch checks the endpoint's network claim, exact sequence, header hashes and
// internal previous-hash links. No independently trusted first-header anchor or
// metadata/transaction-result commitment verification is supplied.
func Fetch(ctx context.Context, rpc RPC, network string, from, to uint32) (Capture, error) {
	out := Capture{SchemaVersion: 1, Kind: "ledger_metadata_capture", Source: "rpc_ledger_meta", Network: network, FromLedger: from, ToLedger: to,
		SequenceCoverage: "PARTIAL", SourceAuthentication: "UNVERIFIED", HistoryCompleteness: "UNKNOWN", MetadataCommitment: "NOT_VERIFIED", StateChanges: "NOT_APPLIED", FinalVerdict: "UNDETERMINED", Ledgers: []Evidence{}, Errors: []string{}}
	fail := func(code string) (Capture, error) {
		out.Errors = append(out.Errors, code)
		return out, errors.New(code)
	}
	if err := ValidateRequest(network, from, to); err != nil {
		return out, err
	}
	if ctx.Err() != nil {
		return fail("CANCELLED")
	}
	n, err := rpc.Network(ctx)
	if err != nil {
		return fail("NETWORK_QUERY_FAILED")
	}
	if n.Passphrase != network {
		return fail("NETWORK_MISMATCH")
	}
	if n.ProtocolVersion < 28 || n.ProtocolVersion > 29 {
		return fail("UNSUPPORTED_NETWORK_PROTOCOL")
	}
	encodedBytes := 0
	for seq := uint64(from); seq <= uint64(to); seq++ {
		if ctx.Err() != nil {
			return fail("CANCELLED")
		}
		page, err := rpc.Ledgers(ctx, protocol.GetLedgersRequest{StartLedger: uint32(seq), Pagination: &protocol.LedgerPaginationOptions{Limit: 1}})
		if err != nil {
			return fail("LEDGER_QUERY_FAILED")
		}
		if page.OldestLedger == 0 || page.LatestLedger < page.OldestLedger || uint32(seq) < page.OldestLedger || uint32(seq) > page.LatestLedger {
			return fail("RETENTION_RANGE_UNAVAILABLE")
		}
		if len(page.Ledgers) != 1 || page.Ledgers[0].Sequence != uint32(seq) {
			return fail("MISSING_OR_UNEXPECTED_LEDGER")
		}
		info := page.Ledgers[0]
		if len(info.LedgerHeader)+len(info.LedgerMetadata) > maxEvidenceBytes-encodedBytes {
			return fail("EVIDENCE_BYTE_LIMIT")
		}
		evidence, err := Decode(info)
		if err != nil {
			return fail("INVALID_OR_UNSUPPORTED_LEDGER_METADATA")
		}
		if len(out.Ledgers) > 0 && evidence.PreviousHash != out.Ledgers[len(out.Ledgers)-1].Hash {
			return fail("HEADER_CHAIN_DIVERGENCE")
		}
		if ctx.Err() != nil {
			return fail("CANCELLED")
		}
		encodedBytes += len(info.LedgerHeader) + len(info.LedgerMetadata)
		out.Ledgers = append(out.Ledgers, evidence)
		out.ProcessedThrough = uint32(seq)
	}
	out.SequenceCoverage = "COMPLETE_ENDPOINT_SEQUENCE"
	return out, nil
}

// Decode checks structural XDR and mutually consistent header claims. A matching
// embedded header does not prove the rest of the metadata belongs to that ledger.
func Decode(info protocol.LedgerInfo) (Evidence, error) {
	var empty Evidence
	if info.Sequence == 0 || info.LedgerCloseTime <= 0 || len(info.LedgerHeader) == 0 || len(info.LedgerHeader) > 8192 || len(info.LedgerMetadata) == 0 || len(info.LedgerMetadata) > base64.StdEncoding.EncodedLen(MaxMetadataBytes) {
		return empty, errors.New("ledger data outside local bounds")
	}
	decode := func(encoded string, target any, maxBytes int64) ([]byte, error) {
		raw, err := base64.StdEncoding.Strict().DecodeString(encoded)
		if err != nil || base64.StdEncoding.EncodeToString(raw) != encoded {
			return nil, errors.New("noncanonical base64 XDR")
		}
		if err := xdr.SafeUnmarshalBase64WithOptions(encoded, target, xdr.DecodeOptions{MaxDepth: 64, MaxMemoryBytes: maxBytes}); err != nil {
			return nil, err
		}
		return raw, nil
	}
	var headerEntry xdr.LedgerHeaderHistoryEntry
	headerRaw, err := decode(info.LedgerHeader, &headerEntry, 16<<10)
	if err != nil {
		return empty, err
	}
	header := headerEntry.Header
	if uint32(header.LedgerSeq) != info.Sequence || uint64(header.ScpValue.CloseTime) != uint64(info.LedgerCloseTime) || header.LedgerVersion < 28 || header.LedgerVersion > 29 {
		return empty, errors.New("unsupported or inconsistent header")
	}
	canonicalHeader, err := header.MarshalBinary()
	if err != nil {
		return empty, err
	}
	headerHash := sha256.Sum256(canonicalHeader)
	claimedHash, err := hex.DecodeString(info.Hash)
	if err != nil || (!bytes.Equal(claimedHash, headerHash[:]) || !bytes.Equal(headerEntry.Hash[:], headerHash[:])) {
		return empty, errors.New("header hash mismatch")
	}
	var meta xdr.LedgerCloseMeta
	metaRaw, err := decode(info.LedgerMetadata, &meta, 16<<20)
	if err != nil || len(metaRaw) > MaxMetadataBytes {
		return empty, errors.New("invalid or oversized metadata XDR")
	}
	var embedded xdr.LedgerHeaderHistoryEntry
	switch {
	case meta.V == 0 && meta.V0 != nil:
		embedded = meta.V0.LedgerHeader
	case meta.V == 1 && meta.V1 != nil:
		embedded = meta.V1.LedgerHeader
	case meta.V == 2 && meta.V2 != nil:
		embedded = meta.V2.LedgerHeader
	default:
		return empty, errors.New("unsupported metadata variant")
	}
	embeddedRaw, err := embedded.MarshalBinary()
	if err != nil || !bytes.Equal(embeddedRaw, headerRaw) || !bytes.Equal(embedded.Hash[:], headerHash[:]) {
		return empty, errors.New("embedded metadata header mismatch")
	}
	metaHash := sha256.Sum256(metaRaw)
	return Evidence{Sequence: info.Sequence, ProtocolVersion: uint32(header.LedgerVersion), CloseTime: info.LedgerCloseTime,
		Hash: hex.EncodeToString(headerHash[:]), PreviousHash: hex.EncodeToString(header.PreviousLedgerHash[:]), MetadataVersion: meta.V, MetadataSHA256: hex.EncodeToString(metaHash[:]), HeaderXDR: info.LedgerHeader, MetadataXDR: info.LedgerMetadata}, nil
}
