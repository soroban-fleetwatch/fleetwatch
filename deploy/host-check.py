#!/usr/bin/env python3
"""Offline, read-only checks for an installed experimental Linux pilot."""
import argparse
import json
import os
import pathlib
import platform
import shutil
import ssl
import subprocess


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("install_root", type=pathlib.Path)
    parser.add_argument("--min-free-bytes", type=int, default=1 << 30,
                        help="required free filesystem bytes (default: 1 GiB)")
    args = parser.parse_args()
    if args.min_free_bytes < 1 << 20:
        parser.error("minimum free bytes must be at least 1 MiB")
    root = args.install_root.resolve()
    checks = []

    def record(name, state, detail):
        checks.append(dict(check=name, state=state, detail=detail))

    def run(command):
        # Never echo subprocess output: config errors and manager environment can
        # contain private endpoint details. No network command is invoked here.
        try:
            return subprocess.run(command, stdin=subprocess.DEVNULL,
                                  stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                                  timeout=15, check=False).returncode == 0
        except (OSError, subprocess.TimeoutExpired):
            return False

    machine_arch = {"x86_64": "amd64", "amd64": "amd64", "aarch64": "arm64", "arm64": "arm64"}.get(platform.machine().lower())
    try:
        with (root / "BUILD.json").open("rb") as metadata:
            raw_metadata = metadata.read(16385)
        if len(raw_metadata) > 16384:
            raise ValueError("oversized build metadata")
        build = json.loads(raw_metadata)
        expected_arch = build["arch"]
        metadata_valid = build["os"] == "linux" and expected_arch in ("amd64", "arm64")
        if not metadata_valid:
            expected_arch = None
    except (OSError, ValueError, KeyError, TypeError):
        expected_arch = None
        metadata_valid = False
    supported = metadata_valid and platform.system() == "Linux" and machine_arch == expected_arch
    record("platform", "PASS" if supported else "FAIL",
           dict(required_os="linux", package_arch=expected_arch, host_arch=machine_arch,
                metadata_valid=metadata_valid))
    tools = all(shutil.which(name) for name in ("bash", "sha256sum", "realpath", "tar"))
    record("tools", "PASS" if tools else "FAIL", "Bash, coreutils and tar required; Python is running")
    cert = ssl.get_default_verify_paths()
    ca_found = bool((cert.cafile and pathlib.Path(cert.cafile).is_file()) or
                    (cert.capath and pathlib.Path(cert.capath).is_dir()))
    for variable, kind in (("SSL_CERT_FILE", "file"), ("SSL_CERT_DIR", "dir")):
        if os.environ.get(variable):
            paths = os.environ[variable].split(":") if kind == "dir" else [os.environ[variable]]
            ca_found = ca_found and any(
                pathlib.Path(value).is_file() if kind == "file" else pathlib.Path(value).is_dir()
                for value in paths)
    record("ca_paths", "PASS" if ca_found else "FAIL",
           "CA paths exist; certificate validity and service TLS connectivity remain untested")
    config = root / "etc/fleetwatch/follow.json"
    binary = root / "bin/fleetwatch"
    valid = supported and run([str(binary), "follow", "--check", "--config", str(config)])
    record("configuration", "PASS" if valid else "FAIL", "offline binary/config/baseline validation")
    state_dir = root / "var/lib/fleetwatch"
    try:
        owned = state_dir.is_dir() and state_dir.stat().st_uid == os.geteuid()
        private = owned and state_dir.stat().st_mode & 0o077 == 0
        writable = os.access(state_dir, os.W_OK | os.X_OK)
        free = shutil.disk_usage(state_dir).free
        record("state_directory", "PASS" if private and writable else "FAIL",
               "requires private current-user-owned writable state directory")
        record("free_disk", "PASS" if free >= args.min_free_bytes else "FAIL",
               dict(available_bytes=free, required_bytes=args.min_free_bytes))
    except OSError:
        record("state_directory", "FAIL", "state directory unavailable")
    unit = root / "systemd/fleetwatch.service"
    static_ok = shutil.which("systemd-analyze") and run(["systemd-analyze", "--user", "verify", str(unit)])
    record("unit_syntax", "PASS" if static_ok else "WARN", "static verification only; no unit started")
    manager_ok = shutil.which("systemctl") and run(["systemctl", "--user", "show", "--property=Version"])
    record("user_manager", "PASS" if manager_ok else "WARN",
           "manager reachable; boot persistence and hardening still require host validation")
    record("deployment_gate", "WARN",
           "RPC, source retention, filesystem type, memory/cgroup limits, backup storage, service readiness, "
           "logout/reboot and notification delivery require separate checks")
    result = "FAIL" if any(c["state"] == "FAIL" for c in checks) else "WARN"
    print(json.dumps(dict(schema_version=1, offline=True, result=result, checks=checks,
                          production_validated=False), indent=2))
    return 1 if result == "FAIL" else 0


if __name__ == "__main__":
    raise SystemExit(main())
