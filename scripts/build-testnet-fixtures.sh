#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root/contracts/testfixtures"
[[ "$(rustc --version)" == 'rustc 1.99.1 '* ]] || { echo 'Rust 1.99.1 is required' >&2; exit 3; }
[[ "$(stellar --version)" == 'stellar 28.1.0 '* ]] || { echo 'Stellar CLI 28.1.0 is required' >&2; exit 3; }
mkdir -p target/fixtures
stellar contract build --locked --package fleetwatch-fixture-member
cp target/wasm32v1-none/release/fleetwatch_fixture_member.wasm target/fixtures/member-v1.wasm
stellar contract build --locked --package fleetwatch-fixture-member --features v2
cp target/wasm32v1-none/release/fleetwatch_fixture_member.wasm target/fixtures/member-v2.wasm
stellar contract build --locked --package fleetwatch-fixture-owner
cp target/wasm32v1-none/release/fleetwatch_fixture_owner.wasm target/fixtures/owner.wasm
cargo test --locked -p fleetwatch-fixture-owner
sha256sum target/fixtures/*.wasm
