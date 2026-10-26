# ADR 0001: establish the offline baseline

Date: 2026-10-26. Status: accepted for the local prototype.

The selected origin is `github.com/soroban-fleetwatch/fleetwatch`. The initial checkout and remote main both identify commit `5bd76719fc47f30fc80d9129b02acc558e734889`, containing only Apache-2.0 LICENSE. Preserve that license and import the supplied Go starter without the bundled third-party Dripwave skill. Use `github.com/soroban-fleetwatch/fleetwatch` as the module path.

Retain the current task branch and establish the offline comparator first. Preserve schema v1 synthetic fixtures and current CLI exits (0 success, 2 threshold exceeded, 3 invalid input). Unknown library thresholds fail closed; CLI invalid thresholds return 3. Network adapters, historical ingestion, and cryptographic approval verification remain separate future components. This decision does not claim CAP-85 integration or authorize external publication.
