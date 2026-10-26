package source

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func address(n byte) string {
	var raw [32]byte
	raw[0] = n
	id, _ := strkey.Encode(strkey.VersionByteContract, raw[:])
	return id
}
func encoded(t *testing.T, v any) string {
	t.Helper()
	s, err := xdr.MarshalBase64(v)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func instance(t *testing.T, id string, ex xdr.ContractExecutable) Entry {
	key, err := InstanceKey(id)
	if err != nil {
		t.Fatal(err)
	}
	value := xdr.ScVal{Type: xdr.ScValTypeScvContractInstance, Instance: &xdr.ScContractInstance{Executable: ex}}
	return dataEntry(t, key, value)
}
func dataEntry(t *testing.T, key xdr.LedgerKey, val xdr.ScVal) Entry {
	k := key.ContractData
	data := xdr.LedgerEntryData{Type: xdr.LedgerEntryTypeContractData, ContractData: &xdr.ContractDataEntry{Contract: k.Contract, Key: k.Key, Durability: k.Durability, Val: val}}
	return Entry{Key: encoded(t, key), XDR: encoded(t, data), LastModified: 90}
}
func external(t *testing.T) xdr.ContractExecutable {
	a, err := contractAddress(address(2))
	if err != nil {
		t.Fatal(err)
	}
	return xdr.ContractExecutable{Type: xdr.ContractExecutableTypeContractExecutableExternalRef, ExternalRef: &xdr.ContractExecutableExternalRef{ExecutableOwner: a, Tag: xdr.ScString(string([]byte{0xff, 0, 0x61}))}}
}
func TestTypedExecutableVariants(t *testing.T) {
	hash := xdr.Hash{1, 2, 3}
	for _, ex := range []xdr.ContractExecutable{{Type: xdr.ContractExecutableTypeContractExecutableWasm, WasmHash: &hash}, {Type: xdr.ContractExecutableTypeContractExecutableStellarAsset}, external(t)} {
		e := instance(t, address(1), ex)
		c, err := DecodeInstance(address(1), e.XDR)
		if err != nil {
			t.Fatal(err)
		}
		switch ex.Type {
		case xdr.ContractExecutableTypeContractExecutableWasm:
			if c.Executable.WasmHash != hex.EncodeToString(hash[:]) {
				t.Fatal("wrong hash")
			}
		case xdr.ContractExecutableTypeContractExecutableStellarAsset:
			if c.Executable.Kind != "stellar_asset" {
				t.Fatal("wrong asset executable")
			}
		default:
			if c.Executable.TagHex != "ff0061" || c.Executable.Owner != address(2) {
				t.Fatal("opaque reference changed")
			}
		}
	}
}
func TestTypedReferenceAndInvalidEntries(t *testing.T) {
	key, err := ReferenceKey(address(2), "FF0061")
	if err != nil {
		t.Fatal(err)
	}
	raw := xdr.ScBytes(make([]byte, 32))
	raw[0] = 255
	e := dataEntry(t, key, xdr.ScVal{Type: xdr.ScValTypeScvBytes, Bytes: &raw})
	ref, err := DecodeReference(address(2), "ff0061", e.XDR)
	if err != nil || ref.TagHex != "ff0061" || !strings.HasPrefix(ref.WasmHash, "ff") {
		t.Fatalf("%+v %v", ref, err)
	}
	if _, err := DecodeReference(address(3), "ff0061", e.XDR); err == nil {
		t.Fatal("wrong owner accepted")
	}
	if _, err := DecodeReference(address(2), "ff0062", e.XDR); err == nil {
		t.Fatal("wrong tag accepted")
	}
	short := xdr.ScBytes{1}
	bad := dataEntry(t, key, xdr.ScVal{Type: xdr.ScValTypeScvBytes, Bytes: &short})
	if _, err := DecodeReference(address(2), "ff0061", bad.XDR); err == nil {
		t.Fatal("short hash accepted")
	}
	for _, s := range []string{"", "not base64", e.XDR + "AAAA", strings.Repeat("A", maxXDRBytes+1)} {
		if _, err := DecodeReference(address(2), "ff0061", s); err == nil {
			t.Fatal("bad XDR accepted")
		}
	}
	if _, err := InstanceKey("CONTRACT_DEMO_A"); err == nil {
		t.Fatal("synthetic ID accepted as chain address")
	}
}

type mockRPC struct {
	network Network
	results []Entries
	calls   int
	err     error
}

func (m *mockRPC) Head(ctx context.Context) (Head, error) {
	return Head{Sequence: 100, ProtocolVersion: 28, CloseTime: time.Now().Unix()}, ctx.Err()
}
func (m *mockRPC) Network(ctx context.Context) (Network, error) {
	if err := ctx.Err(); err != nil {
		return Network{}, err
	}
	return m.network, m.err
}
func (m *mockRPC) Entries(ctx context.Context, keys []string) (Entries, error) {
	if err := ctx.Err(); err != nil {
		return Entries{}, err
	}
	if m.calls >= len(m.results) {
		return Entries{}, errors.New("source failed")
	}
	r := m.results[m.calls]
	m.calls++
	return r, nil
}
func mock(results ...Entries) *mockRPC {
	return &mockRPC{network: Network{Passphrase: "test network", ProtocolVersion: 28}, results: results}
}
func TestCaptureSingleResponseAndNetworkChecks(t *testing.T) {
	e := instance(t, address(1), xdr.ContractExecutable{Type: xdr.ContractExecutableTypeContractExecutableStellarAsset})
	m := mock(Entries{Entries: []Entry{e}, LatestLedger: 100})
	c, err := CaptureCurrent(context.Background(), m, "test network", []string{address(1)})
	if err != nil || c.Completeness != "COMPLETE" || c.LedgerConsistency != "SINGLE_RESPONSE_CLAIM" || c.HistoryCompleteness != "UNKNOWN" || c.SourceAuthentication != "UNVERIFIED" || len(c.Evidence) != 1 {
		t.Fatalf("%+v %v", c, err)
	}
	for _, network := range []Network{{Passphrase: "wrong", ProtocolVersion: 28}, {Passphrase: "test network", ProtocolVersion: 27}} {
		m := mock()
		m.network = network
		if _, err := CaptureCurrent(context.Background(), m, "test network", []string{address(1)}); err == nil || m.calls != 0 {
			t.Fatal("network guard failed")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := CaptureCurrent(ctx, mock(), "test network", []string{address(1)}); err == nil {
		t.Fatal("cancellation ignored")
	}
}
func TestMultiCallCaptureNeverClaimsConsistency(t *testing.T) {
	e := instance(t, address(1), external(t))
	key, _ := ReferenceKey(address(2), "ff0061")
	raw := xdr.ScBytes(make([]byte, 32))
	ref := dataEntry(t, key, xdr.ScVal{Type: xdr.ScValTypeScvBytes, Bytes: &raw})
	for _, height := range []uint32{100, 101} {
		m := mock(Entries{Entries: []Entry{e}, LatestLedger: 100}, Entries{Entries: []Entry{ref}, LatestLedger: height})
		c, err := CaptureCurrent(context.Background(), m, "test network", []string{address(1)})
		if err != nil || c.Completeness != "PARTIAL" || len(c.Errors) == 0 || len(c.Snapshot.References) != 1 {
			t.Fatalf("%+v %v", c, err)
		}
		if height == 101 && c.LedgerConsistency != "INCONSISTENT" {
			t.Fatal("mixed ledgers not detected")
		}
		if height == 100 && c.LedgerConsistency != "UNKNOWN" {
			t.Fatal("equal heights treated as atomic")
		}
	}
}
func TestCaptureMissingMalformedAndDuplicateEntries(t *testing.T) {
	e := instance(t, address(1), xdr.ContractExecutable{Type: xdr.ContractExecutableTypeContractExecutableStellarAsset})
	malformed := e
	malformed.XDR = "bogus"
	wrong := instance(t, address(2), xdr.ContractExecutable{Type: xdr.ContractExecutableTypeContractExecutableStellarAsset})
	future := e
	future.LastModified = 101
	for _, entries := range [][]Entry{nil, {malformed}, {e, e}, {wrong}, {future}} {
		c, err := CaptureCurrent(context.Background(), mock(Entries{Entries: entries, LatestLedger: 100}), "test network", []string{address(1)})
		if err != nil || c.Completeness != "PARTIAL" || len(c.Errors) == 0 {
			t.Fatalf("unsafe result: %+v %v", c, err)
		}
	}
}
func FuzzInstanceXDR(f *testing.F) {
	f.Add("AAAA")
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 1<<16 {
			t.Skip()
		}
		_, _ = DecodeInstance(address(1), s)
	})
}
