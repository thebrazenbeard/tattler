# Tattler donor and public-repository research

Research cut: 2026-10-04. Repositories below were inspected as architecture/provenance inputs. V0.1 currently uses only the Go standard library at runtime; no third-party source is vendored.

## Portfolio mechanisms reused

- `thebrazenbeard/vera-synology@d13cdefbf817aeba60c673fa4a517575c311da2c`: DS216/DSM 7 `armada38x` packaging, loopback-only exposure, lower-privilege package lifecycle, deterministic SPK build/verification, and strict separation of source/build/install/runtime claims.
- `thebrazenbeard/workbridge@e88e14ea25f25abd723bb50909b3e25e67f889fd`: static Go ARMv7 build discipline, fail-closed package verification, bounded/no-authority defaults, and explicit readiness versus live qualification.
- `thebrazenbeard/pre-active@4558923a5ddbb2672449addc424a21de9c62e7ec`: append-only journal semantics and explicit source/effect state rather than silent inference.
- `thebrazenbeard/ingest@27764c9fb97c84d178a3f66e0da2d669df645ed6`: deterministic identity/provenance thinking and the rule that an observation receipt does not promote itself into semantic truth.

These are mechanism donors, not copied implementations.

## Public host-diagnostics research

- `henrygd/beszel@ec96c134660c998f7eb044e349e3715b65733fe1`: lightweight host monitoring was used as a scope/performance reference for a small always-on agent rather than a heavyweight observability stack.
- `prometheus/node_exporter@60ce437b9c737c3b9a9619df457ae68a067d3cfc`: pluggable Linux kernel/OS collectors informed the split between independent metric collectors and the presentation layer. Tattler does not embed Prometheus or expose the entire node_exporter metric surface.
- `AnalogJ/scrutiny@043758c644b91bb04d19a140eb656dfd49d82b5c`: its focused SMART collector/history model supports a future optional storage-health lane instead of pretending raw kernel I/O counters establish drive health.
- `nicolargo/glances@319b7045f842b8b387e4bfa88b108c643464c1aa` and `aristocratos/btop@d3389d74e88644c50907e84314c3c7c973ba5be9`: broad system-monitor surfaces were reviewed as operator-UX references; their richer interactive scope is intentionally reduced for DS216.
- `SynoCommunity/spksrc@00052786a00a4c3cc6b1eaaf7bf7495031bba1bb`: DSM 7 package-account/privilege conventions. The default is package-user execution; Tattler accepts incomplete privileged attribution instead of silently escalating the whole service.

The implemented system sampler independently parses `/proc/stat`, `loadavg`, `meminfo`, `vmstat`, `diskstats`, and selected per-process files. The design intentionally distinguishes gauges from deltas: swap occupancy alone is not swap churn, and cumulative disk counters are converted to rates only across a measured interval.

## Public network-observability research

- `imsnif/bandwhich@1899870cea8bb377948f05cfea64733ec6ea2cd6` (MIT): packet-size capture cross-referenced with `/proc` for process attribution. Useful model; rejected as the DS216 V0.1 runtime because packet capture raises privilege and overhead.
- `raboof/nethogs@459a786ca4c7c3d2011b7a4c84bf6b41586dcf8b` (GPL-2.0-or-later): mature per-process traffic accounting using `/proc` plus packet capture. It reinforces the unavoidable unknown-process race; no code is embedded.
- `cakturk/go-netstat@e5b49efee7a56a0bd9151bd109ab672f189b654d` (MIT): compact `/proc/net` socket-table parsing. Tattler independently implements the same kernel-file approach.
- `ti-mo/conntrack@12267aca6c2453519408b4dee7669603f1ef1183` (MIT): event-driven Netfilter conntrack create/update/destroy listening over Netlink. This is the preferred future high-fidelity Linux connection collector where the kernel and granted capability permit it.
- `evilsocket/opensnitch@a1353848ba1b660320e90cefea782c3fba272c00` (GPL-3.0): interactive application firewall and multi-node observability. Enforcement is deliberately outside V0.1.
- `osquery/osquery@f0af4d842632aee4925d8360920267a8bc726b7c`: structured host instrumentation including open connections/listeners. Its queryable-state model informs Tattler's read-only API shape; osquery itself is too large for the DS216 target.
- `cilium/hubble@90adec13bd28db3eb2671dd0629116640e21b26f` (Apache-2.0): eBPF-backed flow visibility and service maps. Valuable capable-host reference, not a DS216 backend.
- `ntop/ntopng@c09db977d59d6e28139754a1cde8022bc47a32ba` (GPL-3.0): full web traffic analytics. Useful scope reference but substantially heavier than the intended always-on agent.

Additional public categories surveyed included Falco/Tracee-style kernel event monitoring and other Netfilter/pcap process-attribution projects. They reinforce the split between low-overhead sampling on old NAS hardware and high-fidelity kernel/packet telemetry on capable hosts.

## Accepted design conclusions

1. Tattler is a diagnostic agent first; network/socket observation is one diagnostic lane, not the whole product.
2. Core sample/event/findings contracts remain independent of raw `/proc` parsing so collectors can evolve without rewriting the UI.
3. DS216 V0.1 prefers low memory, static binaries, and no third-party runtime dependencies over perfect capture completeness.
4. Process attribution is best-effort evidence with a stated ceiling.
5. Rate findings must come from counter deltas; cumulative or occupied state is not silently promoted into current pressure.
6. Connection metadata is sensitive; UI/API defaults to loopback only.
7. SMART, conntrack, pcap/eBPF, remote export, and remediation are separate later capabilities with their own permission and evidence gates.
