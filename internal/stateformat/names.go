// Package stateformat names recognized on-disk root buckets. Recognition is
// not semantic validation; each engine must still revalidate its own records.
package stateformat

const Replay = "fleetwatch.offline-replay.v1"
const Follow = "fleetwatch.follow.v1"
