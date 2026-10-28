# Changelog

All notable changes are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/). There is no tagged
release yet; everything below is unreleased and experimental.

## [Unreleased]

### Added

- Offline snapshot comparison with blast radius, unsigned policy matching and
  fail-closed severity thresholds (`compare`, `validate`). `c2d0dd9`
- Typed current-state RPC capture with bounded XDR decoding, strict contract
  address checks, freshness checks and bounded retries (`capture`). `c2d0dd9`, `1397b69`
- Offline per-ledger replay with gap detection and durable bbolt checkpoints
  (`replay`, `replay --state`). `d631316`, `dbe6a91`
- Transactional alert outbox, fenced retries, dead letters and HMAC-signed
  HTTPS webhooks with a host allowlist and DNS pinning (`alerts`, `deliver`).
  `5e6670e`, `3f8b120`
- Ed25519 signed approval verification with scoped trust roots, thresholds,
  revocation and rotation, plus durable nonce binding (`verify-approval`,
  `bind-approval`). `9285f0c`, `d968527`
- Read-only health, fleet report and consistent backup commands (`doctor`,
  `report`, `backup`). `09260f9`, `88522f0`, `d8d5c54`
- Bounded ledger metadata collection with header linkage checks, transaction-
  phase-ordered change projection and asserted-baseline reconstruction
  (`history`, `changes`, `reconstruct`). `d9fd859`, `8b71db4`, `bef685f`
- Durable reconstruction checkpoints, stream-scoped alerts, dead-letter
  recovery and resumable RPC ingestion (`checkpoint`, `requeue`, `recoveries`,
  `ingest`, `watch`). `445b6b5` … `4fce115`
- CAP-85 Rust test fixtures and a controlled Testnet validation: shared
  upgrade, rollback and direct repoint observed from real ledger metadata.
  `3eae690`, `85dd0ec`
- Continuous single-host follower with evidence archives, restore, compaction,
  queue sync, a repeated webhook dispatcher and opt-in pinned CONNECT proxy
  routing (`follow`, `follow-*`, `deliver-loop`). `1a5e2a3` … `ebca09c`
- Linux amd64/arm64 packaging, offline installer, systemd units and host
  preflight. `84917cc`, `38ddf56`
- Self-contained browser viewer for current Testnet state. `80c2019`
- Contributor issue templates, pull request template, planned issue catalogue
  with `scripts/create-issues.sh` (dry run by default) and a MkDocs
  documentation site.

### Fixed

- Reject ambiguous RPC JSON (duplicate or case-aliased keys, invalid UTF-8)
  before typed decoding. `981b9e5`
- Update `klauspost/compress` to v1.18.7 for GO-2026-5841 and gate CI on
  vulnerability scans. `542d57d`, `8060f26`
- Recognize an existing outbox format before a writable open, so a foreign
  file is never modified. `d65f200`
- `help`, `--help` and `COMMAND -h` exit 0 instead of the invalid-usage
  status 3. `e879452`

### Security

- No external audit has been performed. Source authentication, metadata
  commitments and fleet completeness remain unverified in every report.
