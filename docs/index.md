# Soroban FleetWatch

FleetWatch is an off-chain monitor for Soroban contract fleets that share code
through CAP-85 executable references. It reads Stellar ledger data, rebuilds
which code each watched contract runs, and reports every change to a shared
reference together with the contracts it affected.

It is a Go command-line tool and a single-host service. It needs no wallet, no
private key and no on-chain contract of its own.

!!! warning "Experimental, not audited"
    Testnet behavior has been demonstrated with real transactions. Every report
    still labels source authentication `UNVERIFIED`, metadata commitments
    `NOT_VERIFIED`, coverage `UNKNOWN` and the final verdict `UNDETERMINED`.
    Do not rely on FleetWatch as the only protection for contracts that hold
    real value. See [Assurance labels](concepts/assurance.md).

## The problem

Protocol 28 ("Adapter") added
[CAP-85](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0085.md).
A contract's executable can now be a reference to an owner contract and an
opaque tag. The owner stores the current Wasm hash for that tag in one
persistent ledger entry. When the owner writes a new hash, every contract that
references the tag runs the new code from that ledger on.

One write upgrades a whole fleet. The same write is also a single point where a
mistaken, malicious or compromised update reaches every member at once. Tools
that check a Wasm build before deployment do not see what actually happened on
the ledger afterwards.

## What FleetWatch does

1. **Reads** ledger-close metadata and ledger entries from a Stellar RPC
   endpoint, with strict JSON and bounded XDR decoding.
2. **Checks** network passphrase, protocol range, header hashes and the link
   from each ledger header to the previous one.
3. **Projects** executable and CAP-85 tag entry changes in transaction-phase order.
4. **Reconstructs** the executable of each watched contract after every
   operation, starting from a baseline you assert.
5. **Reports** reference changes, repoints and observation gaps, with the
   affected contracts, old and new hashes, ledger and evidence digests.
6. **Stores** raw evidence, the cursor and findings in one local transaction,
   so a restart resumes exactly where it stopped.
7. **Delivers** findings to an HTTPS webhook with an HMAC signature, retries
   and an audited dead-letter queue.
8. **Compares** changes with approvals: an unsigned policy file, or Ed25519
   signed statements checked against trust roots you supply. These two are
   always reported separately.

## Where to start

| You want to | Read |
| --- | --- |
| Understand CAP-85 fleets and what can go wrong | [How CAP-85 fleets work](concepts/cap85.md) |
| See how the system is built | [Architecture](ARCHITECTURE.md) |
| Watch your own fleet | [Protocol maintainer guide](guides/maintainer.md) |
| Answer "what ran at ledger N, and who was hit?" | [Security responder guide](guides/responder.md) |
| Check a change against an approval | [Release engineer guide](guides/release-engineer.md) |
| Keep the follower running | [Operator guide](guides/operator.md) |
| Look up a command or exit code | [CLI reference](CLI.md), [Troubleshooting](TROUBLESHOOTING.md) |
| Repeat the Testnet proof | [Testnet demo](TESTNET_DEMO.md) |
| Contribute | [Contributing](contributing.md) |

## Current status

Build progress, the exact commands run and their results are recorded in
[Build status](build/STATUS.md). Open gaps that matter most to users:

- Source data, baselines and metadata commitments are not independently authenticated.
- FleetWatch watches the contracts you list. It does not discover a whole fleet.
- No permanent hosted deployment has been accepted yet.
- No tagged release and no external security review.
