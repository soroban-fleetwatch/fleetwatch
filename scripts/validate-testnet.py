#!/usr/bin/env python3
"""Continue a successful Testnet smoke with real history/restart evidence.

This explicitly writes to the disposable fixture contracts and performs only
local TLS delivery in the separate opt-in Go test. It never changes Mainnet.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import shutil
import subprocess
import time

ROOT = Path(__file__).resolve().parents[1]
NETWORK = "Test SDF Network ; September 2015"
RPC = "https://soroban-testnet.stellar.org"
STREAM = "testnet-demo"


def load(path):
    return json.loads(Path(path).read_text())


def save(path, value):
    with Path(path).open("x") as f:
        json.dump(value, f, indent=2)
        f.write("\n")


def command(args, output, allowed=(0,), env=None):
    with Path(output).open("x") as stdout, Path(str(output) + ".log").open("x") as stderr:
        result = subprocess.run([str(x) for x in args], cwd=ROOT, stdout=stdout,
                                stderr=stderr, env=env, timeout=180)
    if result.returncode not in allowed:
        raise RuntimeError(f"{args[0]} exited {result.returncode}; see {output}.log")
    return result.returncode


def main():
    if not __debug__:
        raise SystemExit("validation requires Python assertions; disable optimized mode")
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("alias")
    parser.add_argument("config_dir", type=Path)
    parser.add_argument("smoke_dir", type=Path)
    parser.add_argument("new_output_dir", type=Path)
    args = parser.parse_args()
    if not shutil.which("stellar") or not shutil.which("go"):
        parser.error("activate the pinned Go and Stellar CLI tools in PATH first")
    if not re.fullmatch(r"[a-z][a-z0-9_-]{0,47}", args.alias):
        parser.error("use a test-only alias, never signing material")
    config = args.config_dir.resolve(strict=True)
    if config.is_relative_to(ROOT):
        parser.error("signing config must be outside the repository")
    smoke = args.smoke_dir.resolve(strict=True)
    out = args.new_output_dir.resolve()
    out.mkdir(mode=0o700)  # Never overwrite a previous run.
    ids = (smoke / "contracts.txt").read_text().splitlines()
    owner = (smoke / "owner.txt").read_text().strip()
    a, b = [(smoke / f"hash-{x}.txt").read_text().strip() for x in ("a", "b")]
    assert len(set(ids)) == 2 and all(re.fullmatch(r"C[A-Z2-7]{55}", x) for x in [owner, *ids])
    assert all(re.fullmatch(r"[0-9a-f]{64}", x) for x in [a, b]) and a != b
    assert load(smoke / "before.json")["snapshot"]["network"] == NETWORK
    cli = out / "fleetwatch"
    receipt = out / "testnet-receipt"
    bridge = out / "testnet-rpc-bridge"
    for name, package in [(cli, "./cmd/fleetwatch"), (receipt, "./scripts/testnet-receipt"), (bridge, "./scripts/testnet-rpc-bridge")]:
        subprocess.run(["go", "build", "-trimpath", "-o", str(name), package], cwd=ROOT, check=True)

    def fw(subcommand, *options, output, allowed=(0,)):
        return command([cli, subcommand, *options], out / output, allowed)

    def history(first, last, filename):
        fw("history", "--rpc-url", RPC, "--network-passphrase", NETWORK,
           "--from-ledger", str(first), "--to-ledger", str(last), output=filename)
        return load(out / filename)

    def baseline_from_capture(capture, stem):
        snapshot = capture["snapshot"]
        pin = history(snapshot["ledger"], snapshot["ledger"], f"{stem}-pin.json")
        baseline = {"schema_version": 1, "kind": "watched_baseline", "source": "user_asserted",
                    "header_hash": pin["ledgers"][0]["hash"], "snapshot": snapshot}
        save(out / f"{stem}-baseline.json", baseline)
        return baseline

    # Verify every previously submitted transaction through the Go read path.
    receipts = {}
    for log in sorted(smoke.glob("*.log")):
        for i, tx in enumerate(dict.fromkeys(re.findall(r"Signing transaction: ([0-9a-f]{64})", log.read_text()))):
            label = f"smoke-{log.stem}-{i}"
            command([receipt, tx], out / f"{label}-receipt.json")
            receipts[label] = load(out / f"{label}-receipt.json")
    assert any(x["transaction_hash"] for k, x in receipts.items() if "publish-b" in k)
    initial = baseline_from_capture(load(smoke / "before.json"), "initial")
    initial_target = load(smoke / "after.json")["snapshot"]["ledger"]
    history(initial["snapshot"]["ledger"] + 1, initial_target, "initial-history.json")
    fw("reconstruct", "--baseline", str(out / "initial-baseline.json"), "--capture", str(out / "initial-history.json"),
       "--state", str(out / "initial.db"), "--stream", STREAM, output="initial-reconstruction.json", allowed=(2,))
    first = load(out / "initial-reconstruction.json")
    assert first["status"] == "COMPLETE_CAPTURE_APPLICATION" and not first["errors"]
    assert any(set(e["finding"].get("affected_contracts", [])) == set(ids)
               for ledger in first["ledgers"] for e in ledger["events"])

    # Open the same DB from fresh processes and prove an already-retained target
    # does not query/write or duplicate the persisted event/outbox identities.
    original_digest = hashlib.sha256((out / "initial.db").read_bytes()).hexdigest()
    fw("ingest", "--rpc-url", RPC, "--baseline", str(out / "initial-baseline.json"),
       "--state", str(out / "initial.db"), "--stream", STREAM, "--to-ledger", str(initial_target), output="initial-restart.json")
    assert load(out / "initial-restart.json")["status"] == "ALREADY_RETAINED"
    assert hashlib.sha256((out / "initial.db").read_bytes()).hexdigest() == original_digest
    print("Initial upgrade receipt/history/restart verified", flush=True)

    with (out / "bridge-url.txt").open("x") as address, (out / "bridge.log").open("x") as log:
        proxy = subprocess.Popen([str(bridge)], stdout=address, stderr=log)
    worker = None
    try:
        for _ in range(50):
            if (out / "bridge-url.txt").stat().st_size:
                break
            if proxy.poll() is not None:
                raise RuntimeError("bridge failed")
            time.sleep(0.1)
        url = (out / "bridge-url.txt").read_text().strip()
        assert re.fullmatch(r"http://127\.0\.0\.1:[0-9]+", url)
        common = ["--no-cache", "--config-dir", str(config), "--rpc-url", url,
                  "--network-passphrase", NETWORK, "--source-account", args.alias, "--inclusion-fee", "100"]

        def invoke(label, contract, function, *options):
            command(["stellar", "contract", "invoke", *common, "--id", contract, "--send=yes", "--", function, *options], out / f"{label}.json")
            txs = re.findall(r"Signing transaction: ([0-9a-f]{64})", (out / f"{label}.json.log").read_text())
            assert len(set(txs)) == 1
            return txs[0]

        (out / "contracts.txt").write_text("\n".join(ids) + "\n")
        fw("capture", "--rpc-url", RPC, "--network-passphrase", NETWORK, "--contracts-file", str(out / "contracts.txt"),
           output="transition-before.json", allowed=(4,))
        before = load(out / "transition-before.json")
        assert before["errors"] == ["multi-call ledger consistency is not proven; capture is diagnostic"]
        assert before["snapshot"]["references"] == [{"owner": owner, "tag_hex": b"fleetwatch-demo".hex(), "wasm_hash": b}]
        baseline = baseline_from_capture(before, "transition")
        target = baseline["snapshot"]["ledger"] + 12
        state = out / "transitions.db"
        save(out / "watch.json", {"schema_version": 1, "rpc_url": RPC, "baseline": "transition-baseline.json",
                                  "state": "transitions.db", "stream": STREAM, "to_ledger": target,
                                  "poll_interval_seconds": 1, "max_attempts": 120, "max_duration_seconds": 120})

        def start_worker(label):
            with (out / f"{label}.ndjson").open("x") as stdout, (out / f"{label}.log").open("x") as stderr:
                return subprocess.Popen([str(cli), "watch", "--config", str(out / "watch.json")], stdout=stdout, stderr=stderr)

        worker = start_worker("watch-before-stop")
        for _ in range(200):
            lines = (out / "watch-before-stop.ndjson").read_text().splitlines()
            if lines and json.loads(lines[-1])["accepted_ledgers"] > 0:
                break
            if worker.poll() is not None:
                raise RuntimeError("watch stopped before cancellation exercise")
            time.sleep(0.1)
        assert lines and json.loads(lines[-1])["status"] == "RETRYING"
        assert json.loads(lines[-1])["accepted_ledgers"] > 0
        worker.send_signal(signal.SIGTERM)
        assert worker.wait(timeout=30) == 4
        fw("doctor", "--state", str(state), "--network", NETWORK, "--source", "rpc_ledger_meta", "--stream", STREAM,
           output="doctor-after-stop.json", allowed=(4,))
        print("Future-range availability gap and SIGTERM checkpoint recorded", flush=True)
        worker = start_worker("watch-resumed")

        pending = {}
        pending["rollback"] = invoke("rollback", owner, "publish", "--tag", "fleetwatch-demo", "--wasm_hash", a)
        for i, member in enumerate(ids):
            pending[f"rollback-version-{i}"] = invoke(f"rollback-version-{i}", member, "version")
        pending["repoint"] = invoke("repoint", ids[0], "repoint", "--wasm_hash", a)
        pending["republish-b"] = invoke("republish-b", owner, "publish", "--tag", "fleetwatch-demo", "--wasm_hash", b)
        for i, member in enumerate(ids):
            pending[f"detached-version-{i}"] = invoke(f"detached-version-{i}", member, "version")
        for label, tx in pending.items():
            command([receipt, tx], out / f"{label}-receipt.json")
            receipts[label] = load(out / f"{label}-receipt.json")
            assert baseline["snapshot"]["ledger"] < receipts[label]["ledger"] <= target
            if "version" in label:
                expected = 1 if label.startswith("rollback") or label.endswith("-0") else 2
                assert load(out / f"{label}.json") == expected
                assert receipts[label]["return_value"]["U32"] == expected
        assert worker.wait(timeout=120) == 2
        worker = None
        history(baseline["snapshot"]["ledger"] + 1, target, "transition-history.json")
        fw("reconstruct", "--baseline", str(out / "transition-baseline.json"), "--capture", str(out / "transition-history.json"),
           output="transition-reconstruction.json", allowed=(2,))
        report = load(out / "transition-reconstruction.json")
        assert report["status"] == "COMPLETE_CAPTURE_APPLICATION" and not report["errors"]
        findings = [e["finding"] for ledger in report["ledgers"] for e in ledger["events"]]
        assert any(f.get("old_hash") == b and f.get("new_hash") == a and set(f.get("affected_contracts", [])) == set(ids) for f in findings)
        assert any(f["code"] == "CONTRACT_EXECUTABLE_REPOINTED" and f["contract_id"] == ids[0] for f in findings)
        assert any(f.get("old_hash") == a and f.get("new_hash") == b and f.get("affected_contracts") == [ids[1]] for f in findings)
        scope = ["--state", str(state), "--network", NETWORK, "--source", "rpc_ledger_meta", "--stream", STREAM]
        fw("doctor", *scope, output="doctor-complete.json", allowed=(0, 2))
        fw("alerts", *scope, output="alerts-before-restart.json")
        fw("watch", "--config", str(out / "watch.json"), output="watch-after-complete.ndjson", allowed=(0, 2))
        fw("alerts", *scope, output="alerts-after-restart.json")
        assert load(out / "alerts-before-restart.json") == load(out / "alerts-after-restart.json")
        print("Rollback/direct repoint and resumed bounded watcher verified", flush=True)
        gap_env = os.environ.copy()
        gap_env["FLEETWATCH_TESTNET_BASELINE"] = str(out / "transition-baseline.json")
        gap_env["FLEETWATCH_TESTNET_CAPTURE"] = str(out / "transition-history.json")
        gap_env["FLEETWATCH_TESTNET_GAP_REPORT"] = str(out / "local-gap-recovery.json")
        command(["go", "test", "-count=1", "-run", "^TestLiveTestnetHistoryGapRecovery$", "-v", "./internal/ingest"],
                out / "local-gap-test.txt", env=gap_env)
        # Send a copied real outbox only to the existing verified-TLS test receiver.
        fw("backup", "--state", str(state), "--output", str(out / "local-delivery.db"), output="backup.json")
        env = os.environ.copy()
        env["FLEETWATCH_TESTNET_STATE"] = str(out / "local-delivery.db")
        env["FLEETWATCH_TESTNET_RECEIVER_REPORT"] = str(out / "local-receiver.json")
        command(["go", "test", "-count=1", "-run", "^TestLiveTestnetOutboxLocalTLS$", "-v", "./internal/alert"], out / "local-delivery-test.txt", env=env)
        save(out / "summary.json", {"network": NETWORK, "owner": owner, "members": ids,
             "initial_upgrade_ledger": initial_target, "transition_baseline_ledger": baseline["snapshot"]["ledger"],
             "processed_through": target, "findings": findings,
             "transactions": {k: {key: v[key] for key in ["transaction_hash", "ledger", "status", "envelope_hash_matches_testnet", "metadata_sha256"]} for k, v in receipts.items()},
             "local_receiver": load(out / "local-receiver.json"),
             "closed_ledger_gap_test": load(out / "local-gap-recovery.json"),
             "source_authentication": "UNVERIFIED", "metadata_commitment": "NOT_VERIFIED", "final_verdict": "UNDETERMINED",
             "gap_exercise": "requested future target unavailable; cancelled and resumed; not a claim of missing closed-chain history"})
        print(f"Scoped live validation recorded in {out}", flush=True)
    finally:
        for process in [worker, proxy]:
            if process is not None and process.poll() is None:
                process.terminate()
                try:
                    process.wait(timeout=30)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()


if __name__ == "__main__":
    os.umask(0o077)
    main()
