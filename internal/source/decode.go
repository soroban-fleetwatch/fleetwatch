// Package source handles untrusted chain data separately from pure comparisons.
package source

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const maxXDRBytes = 2 << 20

func contractAddress(id string) (xdr.ScAddress, error) {
	var a xdr.ScAddress
	raw, err := strkey.Decode(strkey.VersionByteContract, id)
	if err != nil || len(raw) != 32 {
		return a, errors.New("invalid Stellar contract address")
	}
	var hash xdr.ContractId
	copy(hash[:], raw)
	return xdr.NewScAddress(xdr.ScAddressTypeScAddressTypeContract, hash)
}

func InstanceKey(id string) (xdr.LedgerKey, error) {
	a, err := contractAddress(id)
	if err != nil {
		return xdr.LedgerKey{}, err
	}
	var key xdr.LedgerKey
	err = key.SetContractData(a, xdr.ScVal{Type: xdr.ScValTypeScvLedgerKeyContractInstance}, xdr.ContractDataDurabilityPersistent)
	return key, err
}

func ReferenceKey(owner, tagHex string) (xdr.LedgerKey, error) {
	a, err := contractAddress(owner)
	if err != nil {
		return xdr.LedgerKey{}, err
	}
	if len(tagHex) == 0 || len(tagHex) > fleetwatch.MaxTagBytes*2 {
		return xdr.LedgerKey{}, errors.New("tag exceeds local bounds or is empty")
	}
	raw, err := hex.DecodeString(tagHex)
	if err != nil {
		return xdr.LedgerKey{}, errors.New("invalid tag hex")
	}
	tag := xdr.ScString(string(raw))
	var key xdr.LedgerKey
	err = key.SetContractData(a, xdr.ScVal{Type: xdr.ScValTypeScvExecutableTag, ExecutableTag: &tag}, xdr.ContractDataDurabilityPersistent)
	return key, err
}

func decodeEntry(encoded string, expected xdr.LedgerKey) (xdr.ContractDataEntry, error) {
	var empty xdr.ContractDataEntry
	if len(encoded) == 0 || len(encoded) > maxXDRBytes {
		return empty, errors.New("XDR entry outside local size bounds")
	}
	var data xdr.LedgerEntryData
	if err := xdr.SafeUnmarshalBase64WithOptions(encoded, &data, xdr.DecodeOptions{MaxDepth: 64, MaxMemoryBytes: 4 << 20}); err != nil {
		return empty, fmt.Errorf("invalid ledger entry XDR: %w", err)
	}
	if data.Type != xdr.LedgerEntryTypeContractData || data.ContractData == nil || expected.ContractData == nil {
		return empty, errors.New("expected contract data entry")
	}
	entry := *data.ContractData
	want := expected.ContractData
	if !entry.Contract.Equals(want.Contract) || !entry.Key.Equals(want.Key) || entry.Durability != want.Durability {
		return empty, errors.New("ledger entry does not match requested key")
	}
	return entry, nil
}

func DecodeInstance(id, encoded string) (fleetwatch.Contract, error) {
	out := fleetwatch.Contract{ID: id}
	key, err := InstanceKey(id)
	if err != nil {
		return out, err
	}
	entry, err := decodeEntry(encoded, key)
	if err != nil {
		return out, err
	}
	if entry.Val.Type != xdr.ScValTypeScvContractInstance || entry.Val.Instance == nil {
		return out, errors.New("expected contract instance value")
	}
	ex := entry.Val.Instance.Executable
	switch ex.Type {
	case xdr.ContractExecutableTypeContractExecutableWasm:
		if ex.WasmHash == nil {
			return out, errors.New("missing Wasm hash")
		}
		out.Executable = fleetwatch.Executable{Kind: "wasm", WasmHash: hex.EncodeToString(ex.WasmHash[:])}
	case xdr.ContractExecutableTypeContractExecutableStellarAsset:
		out.Executable = fleetwatch.Executable{Kind: "stellar_asset"}
	case xdr.ContractExecutableTypeContractExecutableExternalRef:
		if ex.ExternalRef == nil || ex.ExternalRef.ExecutableOwner.Type != xdr.ScAddressTypeScAddressTypeContract {
			return out, errors.New("external reference owner must be a contract")
		}
		owner, err := ex.ExternalRef.ExecutableOwner.String()
		if err != nil {
			return out, err
		}
		tag := []byte(ex.ExternalRef.Tag)
		if len(tag) == 0 || len(tag) > fleetwatch.MaxTagBytes {
			return out, errors.New("external reference tag outside local bounds")
		}
		out.Executable = fleetwatch.Executable{Kind: "external_ref", Owner: owner, TagHex: hex.EncodeToString(tag)}
	default:
		return out, errors.New("unsupported executable variant")
	}
	return out, nil
}

func DecodeReference(owner, tag, encoded string) (fleetwatch.Reference, error) {
	out := fleetwatch.Reference{Owner: owner}
	key, err := ReferenceKey(owner, tag)
	if err != nil {
		return out, err
	}
	entry, err := decodeEntry(encoded, key)
	if err != nil {
		return out, err
	}
	if entry.Val.Type != xdr.ScValTypeScvBytes || entry.Val.Bytes == nil || len(*entry.Val.Bytes) != 32 {
		return out, errors.New("reference value must be 32-byte Wasm hash")
	}
	raw, _ := hex.DecodeString(tag)
	out.TagHex = hex.EncodeToString(raw)
	out.WasmHash = hex.EncodeToString(*entry.Val.Bytes)
	return out, nil
}
