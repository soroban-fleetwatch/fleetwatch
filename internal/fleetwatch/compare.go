package fleetwatch

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Compare snapshots from an independently managed collector. It does not
// infer transaction authorship, the validity of an approval, or maliciousness.
func Compare(before, after Snapshot, policy Policy) (Report, error) {
	var report Report
	if err := before.Validate(); err != nil {
		return report, fmt.Errorf("before: %w", err)
	}
	if err := after.Validate(); err != nil {
		return report, fmt.Errorf("after: %w", err)
	}
	if err := policy.Validate(); err != nil {
		return report, fmt.Errorf("policy: %w", err)
	}
	if before.Network != after.Network {
		return report, errors.New("snapshots are from different networks")
	}
	if before.Ledger >= after.Ledger {
		return report, errors.New("after ledger must be greater than before ledger")
	}
	report = Report{Coverage: "UNKNOWN", EvaluationStatus: "UNVERIFIED_SNAPSHOTS", SchemaVersion: SchemaVersion, Network: before.Network, BeforeLedger: before.Ledger, AfterLedger: after.Ledger, Findings: []Finding{}, EvidenceDisclaimer: "Findings compare supplied snapshots only; source authenticity, history completeness and on-chain authorization are not verified."}

	beforeRefs, afterRefs := map[string]Reference{}, map[string]Reference{}
	beforeContracts, afterContracts := map[string]Contract{}, map[string]Contract{}
	for _, r := range before.References {
		beforeRefs[refKey(r.Owner, r.TagHex)] = r
	}
	for _, r := range after.References {
		afterRefs[refKey(r.Owner, r.TagHex)] = r
	}
	for _, c := range before.Contracts {
		beforeContracts[c.ID] = c
	}
	for _, c := range after.Contracts {
		afterContracts[c.ID] = c
	}
	approvals := map[string]Approval{}
	for _, a := range policy.ApprovedChanges {
		key := fmt.Sprintf("%s/%s/%s/%s", a.Owner, strings.ToLower(a.TagHex), strings.ToLower(a.FromHash), strings.ToLower(a.ToHash))
		approvals[key] = a
	}

	for key, old := range beforeRefs {
		current, exists := afterRefs[key]
		if !exists {
			report.Findings = append(report.Findings, Finding{Code: "REFERENCE_NOT_OBSERVED", Severity: "high", Message: "Reference absent from newer input. This is an observation gap, not evidence of on-chain deletion.", Owner: old.Owner, TagHex: strings.ToLower(old.TagHex), AffectedContracts: affected(beforeContracts, afterContracts, key)})
			continue
		}
		if strings.EqualFold(old.WasmHash, current.WasmHash) {
			continue
		}
		akey := fmt.Sprintf("%s/%s/%s/%s", old.Owner, strings.ToLower(old.TagHex), strings.ToLower(old.WasmHash), strings.ToLower(current.WasmHash))
		finding := Finding{Code: "UNAPPROVED_REFERENCE_CHANGE", Severity: "critical", Message: "Externally managed executable changed without an exact approved hash transition in the supplied policy.", Owner: old.Owner, TagHex: strings.ToLower(old.TagHex), OldHash: strings.ToLower(old.WasmHash), NewHash: strings.ToLower(current.WasmHash), AffectedContracts: affected(beforeContracts, afterContracts, key)}
		if a, ok := approvals[akey]; ok {
			finding.Code = "POLICY_MATCHED_REFERENCE_CHANGE"
			finding.Severity = "info"
			finding.Message = "Observed executable change exactly matches an approval in the supplied policy; approval authenticity is not verified."
			finding.ApprovalTicket = a.Ticket
		}
		report.Findings = append(report.Findings, finding)
	}
	for key, r := range afterRefs {
		if _, ok := beforeRefs[key]; !ok {
			report.Findings = append(report.Findings, Finding{Code: "NEW_REFERENCE_OBSERVED", Severity: "info", Message: "Reference first observed in the newer snapshot; no upgrade baseline exists.", Owner: r.Owner, TagHex: strings.ToLower(r.TagHex), NewHash: strings.ToLower(r.WasmHash), AffectedContracts: affected(beforeContracts, afterContracts, key)})
		}
	}
	for id, cur := range afterContracts {
		old, exists := beforeContracts[id]
		if !exists {
			report.Findings = append(report.Findings, Finding{Code: "NEW_CONTRACT_OBSERVED", Severity: "info", Message: "Contract first observed in newer snapshot.", ContractID: id})
		} else if !sameExec(old.Executable, cur.Executable) {
			report.Findings = append(report.Findings, Finding{Code: "CONTRACT_EXECUTABLE_REPOINTED", Severity: "high", Message: "Contract executable identity changed (direct Wasm, external owner/tag, or asset executable type).", ContractID: id, OldHash: old.Executable.WasmHash, NewHash: cur.Executable.WasmHash})
		}
		if cur.Executable.Kind == "external_ref" {
			key := refKey(cur.Executable.Owner, cur.Executable.TagHex)
			if _, ok := afterRefs[key]; !ok {
				report.Findings = append(report.Findings, Finding{Code: "UNRESOLVED_EXTERNAL_REFERENCE", Severity: "high", Message: "Executable reference was not resolved in supplied snapshot; include the owner's tag ledger entry.", ContractID: id, Owner: cur.Executable.Owner, TagHex: strings.ToLower(cur.Executable.TagHex)})
			}
		}
	}
	for id := range beforeContracts {
		if _, ok := afterContracts[id]; !ok {
			report.Findings = append(report.Findings, Finding{Code: "CONTRACT_NOT_OBSERVED", Severity: "high", Message: "Previously monitored contract absent from newer input; may be incomplete capture, not on-chain deletion.", ContractID: id})
		}
	}
	for i := range report.Findings {
		report.Findings[i].Authorization = "UNVERIFIED"
	}
	sort.Slice(report.Findings, func(i, j int) bool {
		a, b := report.Findings[i], report.Findings[j]
		rank := map[string]int{"critical": 0, "high": 1, "warning": 2, "info": 3}
		if rank[a.Severity] != rank[b.Severity] {
			return rank[a.Severity] < rank[b.Severity]
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.Owner != b.Owner {
			return a.Owner < b.Owner
		}
		if a.TagHex != b.TagHex {
			return a.TagHex < b.TagHex
		}
		return a.ContractID < b.ContractID
	})
	return report, nil
}

func sameExec(a, b Executable) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case "wasm":
		return strings.EqualFold(a.WasmHash, b.WasmHash)
	case "external_ref":
		return a.Owner == b.Owner && strings.EqualFold(a.TagHex, b.TagHex)
	default:
		return true
	}
}

func affected(before, after map[string]Contract, key string) []string {
	seen := map[string]bool{}
	for _, set := range []map[string]Contract{before, after} {
		for id, c := range set {
			if c.Executable.Kind == "external_ref" && refKey(c.Executable.Owner, c.Executable.TagHex) == key {
				seen[id] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// ShouldFail fails closed for unknown thresholds or finding severities.
// CLI callers reject invalid thresholds as usage errors before comparison.
func ShouldFail(r Report, threshold string) bool {
	levels := map[string]int{"critical": 0, "high": 1, "warning": 2, "info": 3}
	if threshold == "never" {
		return false
	}
	thresholdRank, ok := levels[threshold]
	if !ok {
		return true
	}
	for _, f := range r.Findings {
		rank, known := levels[f.Severity]
		if !known || rank <= thresholdRank {
			return true
		}
	}
	return false
}
