# Tattler roadmap

## V0.1 — passive DS216 diagnostic slice

Implemented and runtime-qualified on the historical DSM subject:

- Linux system/process telemetry for load, CPU, I/O wait, memory, swap, major faults, and top processes;
- one-second sampled socket-table observations with best-effort process/owner attribution;
- bounded evidence-bearing findings;
- append-only rotated JSONL event history;
- loopback-only API/dashboard;
- static ARMv7 DSM package running as the package account.

The live DS216 remains on `0.1.0-0007`. Later source candidates are not installed merely because they build or pass CI.

## V0.2 — storage evidence, typed network observations, and Windows companion

Current source/package candidate: `0.2.0-0002`.

Implemented in source:

- per-physical-disk Linux evidence from `/proc/diskstats`: throughput, IOPS, average completion latency, utilization, and weighted queue depth;
- Linux MD state from `/proc/mdstat`, including bounded degradation evidence and rebuild/resync progress;
- restart-safe bounded event restoration plus rotation/flush failure hardening;
- public typed network observations: `tcp_session`, `tcp_listener`, and `udp_endpoint`;
- native Windows TCP session/listener collection with owning PID;
- native Windows IPv4/IPv6 UDP endpoint collection with owning PID and no invented remote peer;
- native Windows host CPU and memory evidence;
- Windows AMD64 agent build/runtime smoke path;
- separate Wails v2 Windows desktop companion consuming the loopback API;
- deterministic DSM package build and strict package-source provenance gate;
- public-release licensing, security/contribution files, and source hygiene checks.

Release qualification still requires exact-head GitHub Actions success and package-source rebinding to the exact CI-built SPK. DSM installation/runtime qualification remains separate and is not part of source acceptance.

Still pending later V0.2 work:

- DSM-specific device/volume identity;
- optional low-frequency SMART evidence;
- additional source-specific storage context where it materially improves diagnosis.

## V0.3 — durable diagnostic history

Persist compact downsampled system history and finding transitions without writing every raw sample. Add restart-safe retention/export so an operator can inspect the minutes preceding a stall.

## V0.4 — higher-fidelity network events

Add collector interfaces for evidence that is stronger than sampled endpoint tables. Candidate surfaces include Netfilter conntrack on capable Linux hosts and ETW on Windows. Evaluate pcap/eBPF only where platform capability and privilege policy make them appropriate.

Any higher-fidelity collector must preserve provenance and loss/overflow counters. It must not silently upgrade sampled `open`/`close` events into packet or kernel-lifecycle claims.

## V0.5 — richer correlation

Correlate process CPU/RSS/I/O, typed network activity, storage wait, and finding windows. Add optional DNS correlation only with explicit raw-vs-derived provenance.

## V0.6 — multi-host view

Add authenticated export/aggregation as a separate surface. Local-only operation remains valid. Remote transport must not be enabled merely because an aggregator component exists.

## Non-goals unless explicitly added

Tattler is not a firewall, packet-content recorder, remote administration agent, or root-cause oracle. A finding states the strongest conclusion supported by the sampled host evidence.
