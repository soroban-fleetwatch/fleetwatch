package history

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func ledgerFixture(t testing.TB, seq uint32, previous xdr.Hash, version int32) protocol.LedgerInfo {
	t.Helper()
	header := xdr.LedgerHeader{LedgerVersion: 29, LedgerSeq: xdr.Uint32(seq), PreviousLedgerHash: previous, ScpValue: xdr.StellarValue{CloseTime: xdr.TimePoint(1700000000 + seq)}}
	if version == 0 {
		header.LedgerVersion = 28
	}
	raw, err := header.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(raw)
	entry := xdr.LedgerHeaderHistoryEntry{Header: header, Hash: xdr.Hash(hash)}
	var meta xdr.LedgerCloseMeta
	txset := xdr.GeneralizedTransactionSet{V: 1, V1TxSet: &xdr.TransactionSetV1{PreviousLedgerHash: previous}}
	switch version {
	case 0:
		meta = xdr.LedgerCloseMeta{V: 0, V0: &xdr.LedgerCloseMetaV0{LedgerHeader: entry, TxSet: xdr.TransactionSet{PreviousLedgerHash: previous}}}
	case 1:
		meta = xdr.LedgerCloseMeta{V: 1, V1: &xdr.LedgerCloseMetaV1{LedgerHeader: entry, TxSet: txset}}
	case 2:
		meta = xdr.LedgerCloseMeta{V: 2, V2: &xdr.LedgerCloseMetaV2{LedgerHeader: entry, TxSet: txset}}
	default:
		t.Fatal("invalid fixture version")
	}
	headerXDR, err := xdr.MarshalBase64(entry)
	if err != nil {
		t.Fatal(err)
	}
	metadataXDR, err := xdr.MarshalBase64(meta)
	if err != nil {
		t.Fatal(err)
	}
	return protocol.LedgerInfo{Sequence: seq, Hash: hex.EncodeToString(hash[:]), LedgerCloseTime: int64(header.ScpValue.CloseTime), LedgerHeader: headerXDR, LedgerMetadata: metadataXDR}
}

type fakeRPC struct {
	network  source.Network
	infos    []protocol.LedgerInfo
	calls    []protocol.GetLedgersRequest
	change   func(*protocol.GetLedgersResponse)
	netErr   bool
	queryErr bool
}

func (r *fakeRPC) Network(context.Context) (source.Network, error) {
	if r.netErr {
		return source.Network{}, errors.New("sensitive upstream text")
	}
	return r.network, nil
}
func (r *fakeRPC) Ledgers(_ context.Context, req protocol.GetLedgersRequest) (protocol.GetLedgersResponse, error) {
	r.calls = append(r.calls, req)
	if r.queryErr {
		return protocol.GetLedgersResponse{}, errors.New("sensitive upstream text")
	}
	out := protocol.GetLedgersResponse{OldestLedger: 100, LatestLedger: 200}
	for _, info := range r.infos {
		if info.Sequence == req.StartLedger {
			out.Ledgers = append(out.Ledgers, info)
		}
	}
	if r.change != nil {
		r.change(&out)
	}
	return out, nil
}
func historyFixture(t testing.TB) *fakeRPC {
	a := ledgerFixture(t, 100, xdr.Hash{}, 0)
	raw, _ := hex.DecodeString(a.Hash)
	var hash xdr.Hash
	copy(hash[:], raw)
	b := ledgerFixture(t, 101, hash, 2)
	return &fakeRPC{network: source.Network{Passphrase: "SYNTHETIC-NETWORK", ProtocolVersion: 29}, infos: []protocol.LedgerInfo{a, b}}
}

func TestFetchSequentialMetadataAndAssurance(t *testing.T) {
	rpc := historyFixture(t)
	r, err := Fetch(context.Background(), rpc, rpc.network.Passphrase, 100, 101)
	if err != nil || r.SequenceCoverage != "COMPLETE_ENDPOINT_SEQUENCE" || r.ProcessedThrough != 101 || len(r.Ledgers) != 2 {
		t.Fatalf("%+v %v", r, err)
	}
	if r.SourceAuthentication != "UNVERIFIED" || r.HistoryCompleteness != "UNKNOWN" || r.MetadataCommitment != "NOT_VERIFIED" || r.StateChanges != "NOT_APPLIED" || r.FinalVerdict != "UNDETERMINED" {
		t.Fatalf("%+v", r)
	}
	for i, req := range rpc.calls {
		if req.StartLedger != uint32(100+i) || req.Pagination == nil || req.Pagination.Limit != 1 || req.Pagination.Cursor != "" {
			t.Fatalf("%+v", req)
		}
	}
	for i, e := range r.Ledgers {
		raw, _ := base64.StdEncoding.DecodeString(rpc.infos[i].LedgerMetadata)
		hash := sha256.Sum256(raw)
		if e.MetadataSHA256 != hex.EncodeToString(hash[:]) || e.HeaderXDR != rpc.infos[i].LedgerHeader || e.MetadataXDR != rpc.infos[i].LedgerMetadata {
			t.Fatal("lost raw evidence")
		}
	}
	again, err := Fetch(context.Background(), historyFixture(t), rpc.network.Passphrase, 100, 101)
	if err != nil || !reflect.DeepEqual(r, again) {
		t.Fatal("non-deterministic capture", err)
	}
}

func TestDecodeMetadataVersionsAndTampering(t *testing.T) {
	for _, version := range []int32{0, 1, 2} {
		info := ledgerFixture(t, 100, xdr.Hash{}, version)
		if e, err := Decode(info); err != nil || e.MetadataVersion != version {
			t.Fatal(e, err)
		}
	}
	for _, kind := range []string{"sequence", "time", "hash", "embedded_header", "embedded_hash", "bad_header", "raw_header", "trailing", "noncanonical", "unknown_variant", "oversized", "future_protocol", "old_protocol", "allocation"} {
		t.Run(kind, func(t *testing.T) {
			info := ledgerFixture(t, 100, xdr.Hash{}, 2)
			switch kind {
			case "sequence":
				info.Sequence++
			case "time":
				info.LedgerCloseTime++
			case "hash":
				info.Hash = strings.Repeat("f", 64)
			case "bad_header":
				info.LedgerHeader = "not base64"
			case "raw_header":
				var e xdr.LedgerHeaderHistoryEntry
				_ = xdr.SafeUnmarshalBase64(info.LedgerHeader, &e)
				info.LedgerHeader, _ = xdr.MarshalBase64(e.Header)
			case "trailing":
				b, _ := base64.StdEncoding.DecodeString(info.LedgerMetadata)
				info.LedgerMetadata = base64.StdEncoding.EncodeToString(append(b, 0, 0, 0, 0))
			case "noncanonical":
				info.LedgerMetadata += "\n"
			case "unknown_variant":
				info.LedgerMetadata = base64.StdEncoding.EncodeToString([]byte{0, 0, 0, 9})
			case "oversized":
				info.LedgerMetadata = strings.Repeat("A", base64.StdEncoding.EncodedLen(MaxMetadataBytes)+4)
			case "embedded_header", "embedded_hash":
				var meta xdr.LedgerCloseMeta
				_ = xdr.SafeUnmarshalBase64(info.LedgerMetadata, &meta)
				if kind == "embedded_header" {
					meta.V2.LedgerHeader.Header.LedgerSeq++
				} else {
					meta.V2.LedgerHeader.Hash[0] ^= 1
				}
				info.LedgerMetadata, _ = xdr.MarshalBase64(meta)
			case "future_protocol", "old_protocol":
				var entry xdr.LedgerHeaderHistoryEntry
				_ = xdr.SafeUnmarshalBase64(info.LedgerHeader, &entry)
				entry.Header.LedgerVersion = 30
				if kind == "old_protocol" {
					entry.Header.LedgerVersion = 27
				}
				info.LedgerHeader, _ = xdr.MarshalBase64(entry)
			case "allocation":
				// Replace the v2 txProcessing vector length (after the empty txset)
				// with an allocation bomb. It must fail within decoder budgets.
				var entry xdr.LedgerHeaderHistoryEntry
				_ = xdr.SafeUnmarshalBase64(info.LedgerHeader, &entry)
				entryRaw, _ := entry.MarshalBinary()
				raw, _ := base64.StdEncoding.DecodeString(info.LedgerMetadata)
				offset := 8 + len(entryRaw) + 4 + 32 + 4
				copy(raw[offset:offset+4], []byte{255, 255, 255, 255})
				info.LedgerMetadata = base64.StdEncoding.EncodeToString(raw)
			}
			if _, err := Decode(info); err == nil {
				t.Fatal("accepted invalid metadata")
			}
		})
	}
}

func TestFetchStopsBeforeGapsDivergenceAndFailures(t *testing.T) {
	for _, kind := range []string{"network", "network_error", "protocol", "query_error", "missing", "duplicate", "retention", "wrong_sequence", "divergence"} {
		t.Run(kind, func(t *testing.T) {
			rpc := historyFixture(t)
			switch kind {
			case "network":
				rpc.network.Passphrase = "OTHER"
			case "network_error":
				rpc.netErr = true
			case "protocol":
				rpc.network.ProtocolVersion = 30
			case "query_error":
				rpc.queryErr = true
			case "missing":
				rpc.infos = rpc.infos[:1]
			case "duplicate":
				rpc.change = func(r *protocol.GetLedgersResponse) { r.Ledgers = append(r.Ledgers, r.Ledgers...) }
			case "retention":
				rpc.change = func(r *protocol.GetLedgersResponse) { r.OldestLedger = 101 }
			case "wrong_sequence":
				rpc.change = func(r *protocol.GetLedgersResponse) { r.Ledgers[0].Sequence++ }
			case "divergence":
				rpc.infos[1] = ledgerFixture(t, 101, xdr.Hash{}, 2)
			}
			r, err := Fetch(context.Background(), rpc, "SYNTHETIC-NETWORK", 100, 101)
			if err == nil || r.SequenceCoverage != "PARTIAL" || len(r.Errors) != 1 || strings.Contains(err.Error(), "sensitive") {
				t.Fatalf("%+v %v", r, err)
			}
			if kind == "missing" || kind == "divergence" {
				if r.ProcessedThrough != 100 || len(r.Ledgers) != 1 {
					t.Fatalf("%+v", r)
				}
			} else if r.ProcessedThrough != 0 || len(r.Ledgers) != 0 {
				t.Fatalf("%+v", r)
			}
		})
	}
}

func TestHistoryBoundsCancellationAndMetadataTrustBoundary(t *testing.T) {
	rpc := historyFixture(t)
	for _, p := range [][2]uint32{{0, 1}, {100, 99}, {100, 116}} {
		if _, err := Fetch(context.Background(), rpc, "SYNTHETIC-NETWORK", p[0], p[1]); err == nil {
			t.Fatal("invalid range accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, err := Fetch(ctx, rpc, "SYNTHETIC-NETWORK", 100, 101); err == nil || len(rpc.calls) != 0 || len(r.Ledgers) != 0 {
		t.Fatal("cancellation ignored")
	}
	// Alter non-header metadata while retaining a valid embedded header. Decode
	// cannot authenticate it without verifying the ledger's other commitments.
	info := ledgerFixture(t, 100, xdr.Hash{}, 2)
	var meta xdr.LedgerCloseMeta
	_ = xdr.SafeUnmarshalBase64(info.LedgerMetadata, &meta)
	meta.V2.TotalByteSizeOfLiveSorobanState = 1234
	info.LedgerMetadata, _ = xdr.MarshalBase64(meta)
	if _, err := Decode(info); err != nil {
		t.Fatal("unexpected claim of full metadata commitment verification", err)
	}
}

func FuzzLedgerMetadataDecode(f *testing.F) {
	info := ledgerFixture(f, 100, xdr.Hash{}, 2)
	f.Add(info.LedgerMetadata)
	f.Add("AAAA/w==")
	f.Add("")
	f.Fuzz(func(t *testing.T, encoded string) {
		candidate := info
		candidate.LedgerMetadata = encoded
		_, _ = Decode(candidate)
	})
}
