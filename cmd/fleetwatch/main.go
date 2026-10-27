package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
)

const version = "0.1.0-dev"

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 3
	}
	switch args[0] {
	case "version":
		fmt.Fprintln(stdout, "fleetwatch "+version)
		return 0
	case "validate":
		fs := flag.NewFlagSet("validate", flag.ContinueOnError)
		fs.SetOutput(stderr)
		file := fs.String("snapshot", "", "path to fleet snapshot JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return 3
		}
		if *file == "" || fs.NArg() != 0 {
			fmt.Fprintln(stderr, "validate requires --snapshot path")
			return 3
		}
		s, err := fleetwatch.LoadSnapshot(*file)
		if err != nil {
			fmt.Fprintln(stderr, "invalid snapshot:", err)
			return 3
		}
		fmt.Fprintf(stdout, "valid snapshot: %s ledger=%d contracts=%d references=%d\n", s.Network, s.Ledger, len(s.Contracts), len(s.References))
		return 0
	case "bind-approval":
		return bindApproval(args[1:], stdout, stderr)
	case "verify-approval":
		return verifyApproval(args[1:], stdout, stderr)
	case "deliver":
		return deliver(args[1:], stdout, stderr)
	case "requeue":
		return recovery(args[1:], stdout, stderr, false)
	case "recoveries":
		return recovery(args[1:], stdout, stderr, true)
	case "alerts":
		return alerts(args[1:], stdout, stderr)
	case "doctor":
		return doctor(args[1:], stdout, stderr)
	case "report":
		return reportFleet(args[1:], stdout, stderr)
	case "backup":
		return backup(args[1:], stdout, stderr)
	case "watch":
		return watch(args[1:], stdout, stderr)
	case "follow":
		return followService(args[1:], stdout, stderr)
	case "follow-inspect":
		return followInspect(args[1:], stdout, stderr)
	case "follow-archive":
		return followArchive(args[1:], stdout, stderr, false)
	case "verify-follow-archive":
		return followArchive(args[1:], stdout, stderr, true)
	case "restore-follow-archive":
		return restoreFollowArchive(args[1:], stdout, stderr)
	case "follow-compact":
		return followCompact(args[1:], stdout, stderr)
	case "ingest":
		return ingestHistory(args[1:], stdout, stderr)
	case "history":
		return captureHistory(args[1:], stdout, stderr)
	case "changes":
		return normalizeChanges(args[1:], stdout, stderr)
	case "reconstruct":
		return reconstruct(args[1:], stdout, stderr)
	case "checkpoint":
		return checkpoint(args[1:], stdout, stderr)
	case "replay":
		return replay(args[1:], stdout, stderr)
	case "capture":
		return capture(args[1:], stdout, stderr)
	case "compare":
		return compare(args[1:], stdout, stderr)
	default:
		usage(stderr)
		return 3
	}
}

func compare(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	fs.SetOutput(stderr)
	before := fs.String("before", "", "earlier snapshot JSON")
	after := fs.String("after", "", "later snapshot JSON")
	policyPath := fs.String("policy", "", "optional approval policy JSON; absent means no approved changes")
	output := fs.String("format", "text", "text or json")
	failOn := fs.String("fail-on", "high", "critical, high, warning, info or never")
	if err := fs.Parse(args); err != nil {
		return 3
	}
	if *before == "" || *after == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "compare requires --before and --after paths")
		return 3
	}
	allowed := map[string]bool{"critical": true, "high": true, "warning": true, "info": true, "never": true}
	if (*output != "text" && *output != "json") || !allowed[*failOn] {
		fmt.Fprintln(stderr, "invalid --format or --fail-on option")
		return 3
	}
	b, err := fleetwatch.LoadSnapshot(*before)
	if err != nil {
		fmt.Fprintln(stderr, "invalid before:", err)
		return 3
	}
	a, err := fleetwatch.LoadSnapshot(*after)
	if err != nil {
		fmt.Fprintln(stderr, "invalid after:", err)
		return 3
	}
	policy, err := fleetwatch.LoadPolicy(*policyPath)
	if err != nil {
		fmt.Fprintln(stderr, "invalid policy:", err)
		return 3
	}
	report, err := fleetwatch.Compare(b, a, policy)
	if err != nil {
		fmt.Fprintln(stderr, "comparison failed:", err)
		return 3
	}
	if *output == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			fmt.Fprintln(stderr, err)
			return 3
		}
	} else {
		fmt.Fprintf(stdout, "FleetWatch: %s ledger %d -> %d\n", report.Network, report.BeforeLedger, report.AfterLedger)
		fmt.Fprintf(stdout, "Coverage: %s; evaluation: %s; authorization: UNVERIFIED\n", report.Coverage, report.EvaluationStatus)
		if len(report.Findings) == 0 {
			fmt.Fprintln(stdout, "No changes or observation gaps detected in supplied snapshots.")
		}
		for _, f := range report.Findings {
			fmt.Fprintf(stdout, "[%s] %s: %s\n", strings.ToUpper(f.Severity), f.Code, f.Message)
			if f.Owner != "" {
				fmt.Fprintf(stdout, "  owner=%s tag_hex=%s\n", f.Owner, f.TagHex)
			}
			if f.ContractID != "" {
				fmt.Fprintf(stdout, "  contract=%s\n", f.ContractID)
			}
			if f.OldHash != "" || f.NewHash != "" {
				fmt.Fprintf(stdout, "  old=%s new=%s\n", f.OldHash, f.NewHash)
			}
			if len(f.AffectedContracts) > 0 {
				fmt.Fprintf(stdout, "  affected=%s\n", strings.Join(f.AffectedContracts, ","))
			}
			if f.ApprovalTicket != "" {
				fmt.Fprintf(stdout, "  policy_ticket=%s\n", f.ApprovalTicket)
			}
		}
		fmt.Fprintln(stdout, "NOTE:", report.EvidenceDisclaimer)
	}
	if fleetwatch.ShouldFail(report, *failOn) {
		return 2
	}
	return 0
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "fleetwatch <compare|validate|capture|replay|alerts|requeue|recoveries|doctor|report|backup|history|watch|follow|follow-inspect|follow-archive|verify-follow-archive|restore-follow-archive|follow-compact|ingest|changes|reconstruct|checkpoint|deliver|verify-approval|bind-approval|version>\n  compare --before old.json --after new.json [--policy approvals.json] [--format text|json] [--fail-on critical|high|warning|info|never]\n  validate --snapshot fleet.json\ncapture --rpc-url HTTPS_URL --network-passphrase PASSPHRASE --contracts-file FILE\n  replay --journal per-ledger-snapshots.json\n  alerts --state replay.db --network TEXT [--source synthetic|user_snapshot]\n  requeue --state FILE --network TEXT --alert-id ID --request-id ID --expected-cycle N --reason TEXT\n  recoveries --state FILE --network TEXT --alert-id ID\n  checkpoint --state FILE --network TEXT --stream ID\n  reconstruct --baseline watched-baseline.json --capture history-capture.json [--state FILE --stream ID]\n  changes --capture history-capture.json\n  follow --config FILE [--check]\n  follow-inspect --config FILE [--limit N --after-id ID]\n  follow-archive --config FILE --output NEW_FILE\n  verify-follow-archive --input FILE\n  restore-follow-archive --input FILE --output NEW_DB --sha256 EXPECTED_DIGEST\n  follow-compact --config FILE --output NEW_DB\n  watch --config FILE\n  ingest --rpc-url HTTPS_URL --baseline FILE --state FILE --stream ID --to-ledger N\n  history --rpc-url HTTPS_URL --network-passphrase TEXT --from-ledger N --to-ledger M\n  backup --state replay.db --output NEW_FILE\n  report --state replay.db --network TEXT --ledger N --owner ID --tag-hex HEX\n  doctor --state replay.db --network TEXT [--source synthetic|user_snapshot|rpc_ledger_meta] [--stream ID for rpc_ledger_meta]\n  deliver --state replay.db --network TEXT --webhook-url HTTPS_URL --allow-host HOST\n  verify-approval --approval envelope.json --trust trust.json --observation transition.json\n  bind-approval --state replay.db --network TEXT --event-id ID --approval envelope.json --trust trust.json\nExit 0=pass, 2=policy threshold exceeded, 3=invalid input/usage, 4=source unavailable/incomplete, 5=state/output failure, 6=follow retention capacity")
}
