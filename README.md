# Tattler

Tattler is a low-overhead Linux/NAS diagnostic agent built to answer a practical question:

> Why is this machine slow right now?

The first deployment target is Synology DSM on DS216-class ARMv7 hardware, but the collector and diagnosis engine are portable Linux Go code.

## V0.1 diagnostic surface

Tattler combines two read-only evidence streams:

- system/process telemetry from Linux `/proc`: CPU use, load, I/O wait, memory and swap pressure, major faults, disk throughput, and top processes by CPU/RSS/I/O;
- host connection telemetry from `/proc/net/tcp`, `tcp6`, `udp`, and `udp6`, with best-effort PID/process attribution.

The diagnosis layer currently emits bounded findings such as `memory-pressure`, `swap-churn`, `storage-wait`, `cpu-saturation`, `blocked-load`, and `major-faults`. Findings report the supporting measurement rather than pretending to establish a root cause that the evidence cannot prove.
Connection changes are recorded as versioned `open` / `close` events with host, observation time, collector source, and a deterministic event digest. Those events are persisted to bounded rotated JSONL.

The local dashboard/API exposes:

- `/api/v1/status`
- `/api/v1/system`
- `/api/v1/findings`
- `/api/v1/current`
- `/api/v1/events`

The HTTP surface is restricted to `127.0.0.1:9147` in V0.1. Connection and process history are sensitive host activity, so LAN/public exposure is not enabled by default.

## Performance posture

The DS216 has a very small resource budget. Connection sampling defaults to 1 second, while the more expensive system/process scan defaults to 5 seconds. Tattler has no runtime dependency beyond the Linux kernel interfaces and the Go static binary.

The DSM package runs as its package account rather than root. If DSM permissions prevent reading another process's `/proc/<pid>/fd`, process attribution is incomplete instead of silently escalating authority.
## Evidence ceiling

V0.1 is a sampling diagnostic agent, not a packet sniffer or kernel tracing engine. Very short connections can occur between samples. Unconnected inbound UDP is not represented as a connection. PID attribution can race process exit or be blocked by permissions. Linux namespaces and NAT can also limit what the host view proves.

A future collector can add event-driven Netfilter conntrack where compatible without replacing the event/API contract. Packet/eBPF backends are reserved for hosts where the kernel, privilege model, and hardware budget justify them.

## Research baseline

The implementation reuses architectural mechanisms from `thebrazenbeard/vera-synology`, `workbridge`, `pre-active`, and `ingest`, while public-repository research included SynoCommunity `spksrc`, Beszel, Prometheus node_exporter, Scrutiny, Bandwhich, NetHogs, go-netstat, conntrack, OpenSnitch, osquery, Hubble, Falco/Tracee-class monitoring, and ntopng.

No donor implementation is vendored in V0.1. Exact research cuts and design conclusions are recorded in `docs/RESEARCH.md`.
## Build and verification

```bash
go test ./...
go vet ./...
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 \
  go build -trimpath -buildvcs=false -ldflags='-s -w -buildid=' \
  -o dist/tattler-linux-armv7 ./cmd/tattler

python tools/build_spk.py --binary dist/tattler-linux-armv7 \
  --output dist/Tattler-armada38x-0.1.0-0003.spk
python tools/verify_spk.py dist/Tattler-armada38x-0.1.0-0003.spk
```

Linux runtime:

```bash
./tattler --state-dir ./tattler-state
```

## Status

Current source/package subject `0.1.0-0003`:

`SOURCE_IMPLEMENTED / LOCAL_TEST_PASS / ARMV7_BUILD_PASS / SPK_STRUCTURAL_VERIFY_PASS / NOT INSTALLED / NOT NAS_RUNTIME_QUALIFIED`

Installed predecessor `0.1.0-0002` on DS216 / DSM 7.2.2:

`PACKAGE_CENTER_INSTALL_PASS / LIFECYCLE_START_PASS / LIVE_DAEMON_PASS / LOOPBACK_API_PASS / SYSTEM_TELEMETRY_PASS / CONNECTION_ENDPOINT_VISIBILITY_PASS / CROSS_USER_PROCESS_ATTRIBUTION_LIMITED`

See `docs/DSM_7_2_2_RUNTIME_QUALIFICATION_20261004.md` for the exact live subject, measurements, evidence ceiling, and observed Plex/storage stall.

Source, build, SPK verification, Package Center installation, live DSM service behavior, capture completeness, and diagnostic validity are separate evidence states.
