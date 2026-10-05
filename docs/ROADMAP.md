# Tattler roadmap

## V0.1 — passive DS216 diagnostic slice

Implemented source target:

- five-second Linux system/process telemetry for load, CPU, I/O wait, memory, swap, major faults, disk throughput, and top processes;
- bounded evidence-bearing findings for common pressure patterns;
- one-second `/proc/net` socket sampling with best-effort process attribution and direction classification;
- append-only rotated connection-event JSONL;
- loopback-only API/dashboard;
- static ARMv7 build and DSM 7 `armada38x` SPK scaffold.

Qualification gates before calling the SPK runtime-ready:

- Package Center installs the exact built artifact on DS216 / DSM 7.2.
- Service starts/stops/restarts cleanly as the package account.
- Required `/proc` files are readable under the installed package identity.
- Connection journal survives restart and rotates without data loss.
- Idle CPU/RSS plus one-second socket/five-second system sampling overhead are acceptable on the 512 MB host.
- Controlled CPU, memory, I/O-wait, swap, inbound-TCP, and outbound-TCP fixtures produce the expected measurements/findings/events.
- Live DSM evidence confirms thresholds are useful without causing alert churn.

Source status for the journal portion of this gate: restart restoration, bounded recent-history recovery, rotation error propagation/rollback, close-flush error reporting, and interrupted-rotation recovery-file visibility are implemented and exact-head CI qualified in the `0.2.0-0001` candidate. They are **not** yet DS216 runtime-qualified; the live NAS remains on `0.1.0-0007`.

## V0.2 — storage and DSM evidence

Source candidate `0.2.0-0001` implements the first storage-evidence slice from already-readable `/proc/diskstats`: per-physical-disk throughput, IOPS, average completion latency, utilization, and weighted queue depth. Existing `storage-wait` findings now carry the hottest disk's measured utilization/latency/queue evidence when available.

Still pending runtime qualification and later V0.2 work: DSM/md RAID state, device/volume identity, optional low-frequency SMART evidence, and any additional source-specific collectors. Keep expensive SMART polling far outside the hot path.

## V0.3 — durable diagnostic history

Persist compact downsampled system history and finding transitions without writing every raw sample. Add restart-safe retention and export so an operator can inspect the minutes preceding a stall.

## V0.4 — higher-fidelity connection events

Add a collector interface plus Netfilter conntrack for hosts where kernel and granted capability permit event subscription. Preserve the V1 event envelope and expose loss/overflow counters. Evaluate pcap/eBPF only on capable hosts.

## V0.5 — richer correlation

Correlate process CPU/RSS/I/O, connection activity, storage wait, and finding windows. Add optional UID/user mapping and DNS correlation while preserving raw-vs-derived provenance.

## V0.6 — multi-host view

Add authenticated export/aggregation as a separate surface. Local-only operation remains valid. Remote transport must not be enabled merely because an aggregator component exists.

## Non-goals unless explicitly added

Tattler is not a firewall, packet-content recorder, remote administration agent, or magical root-cause oracle. A finding states the strongest conclusion supported by the sampled host evidence.
