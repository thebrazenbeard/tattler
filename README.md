# Tattler

Tattler is a low-overhead Linux/NAS diagnostic agent built to answer a practical question:

> Why is this machine slow right now?

The first deployment target is Synology DSM on DS216-class ARMv7 hardware, but the collector and diagnosis engine are portable Linux Go code.

## V0.1 diagnostic surface

Tattler combines two read-only evidence streams:

- system/process telemetry from Linux `/proc`: CPU use, load, I/O wait, memory and swap pressure, major faults, disk throughput, and top processes by CPU/RSS/I/O;
- host connection telemetry from `/proc/net/tcp`, `tcp6`, `udp`, and `udp6`, with best-effort PID/process attribution.

The diagnosis layer emits bounded findings such as `memory-pressure`, `swap-churn`, `storage-wait`, `cpu-saturation`, `blocked-load`, and `major-faults`. Findings report the supporting measurement rather than pretending to establish a root cause the evidence cannot prove.

Connection changes are persisted as versioned `open` / `close` JSONL events with host, observation time, collector source, and deterministic event IDs.

The loopback-only dashboard/API exposes:

- `/api/v1/status`
- `/api/v1/system`
- `/api/v1/findings`
- `/api/v1/current`
- `/api/v1/events`

## DSM privilege model

The DSM package itself remains `run-as: package`; Tattler does not request root lifecycle actions.

DSM 7 documents file capabilities for individual package tools. Tattler v0005 therefore moves cross-user socket-to-process attribution into a tiny sibling executable, `bin/tattler-procmap`, and requests only `cap_sys_ptrace` for that helper. The HTTP/API daemon itself receives no capability.

The daemon first resolves process ownership normally. It invokes the helper only for still-unresolved socket inodes, then caches successful mappings. This keeps the elevated surface small and avoids repeatedly scanning all processes when existing mappings are already known.

Whether `cap_sys_ptrace` is sufficient on the DS216's actual DSM 7.2.2 `/proc` policy remains a live-runtime qualification question. The package does not promote that design intent into a success claim until installed and read back.

## Native DSM updates

Tattler uses DSM's native Package Center upgrade path instead of a root self-updater.

The SPK declares:

- `silent_upgrade="yes"`
- `auto_upgrade_from="0.1.0-0003"`

The repository also contains `package-source/`, a small DSM package-source endpoint that implements the NAS catalog shape used by third-party repositories. It serves only `armada38x` hosts at DSM build `72806` or newer and binds each catalog entry to the exact qualified SPK's MD5, size, version, icons, and download URL.

`tools/update_package_source.py` derives that release manifest from the finished SPK rather than maintaining it by hand.

The rejected root-bridge experiment is preserved under `archive/rejected-root-bridge-v0004/` as historical evidence only. It is not part of the active package or CI path.

## Performance posture

The DS216 has a very small resource budget. Connection sampling defaults to 1 second, while the more expensive system/process scan defaults to 5 seconds. The runtime has no third-party Go dependencies.

## Evidence ceiling

V0.1 is a sampling diagnostic agent, not a packet sniffer or kernel tracing engine. Very short connections can occur between samples. Unconnected inbound UDP is not represented as a connection. PID attribution can race process exit or be blocked by permissions. Linux namespaces and NAT can also limit what the host view proves.

A future collector can add event-driven Netfilter conntrack where compatible without replacing the event/API contract. Packet/eBPF backends are reserved for hosts where the kernel, privilege model, and hardware budget justify them.

## Build and verification

```bash
go test ./...
go vet ./...

CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 \
  go build -trimpath -buildvcs=false -ldflags='-s -w -buildid=' \
  -o dist/tattler-linux-armv7 ./cmd/tattler

CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 \
  go build -trimpath -buildvcs=false -ldflags='-s -w -buildid=' \
  -o dist/tattler-procmap-linux-armv7 ./cmd/tattler-procmap

python tools/build_spk.py \
  --binary dist/tattler-linux-armv7 \
  --helper dist/tattler-procmap-linux-armv7 \
  --output dist/Tattler-armada38x-0.1.0-0005.spk

python tools/verify_spk.py dist/Tattler-armada38x-0.1.0-0005.spk
```

## Status

Installed `0.1.0-0003` on DS216 / DSM 7.2.2:

`PACKAGE_CENTER_UPGRADE_PASS / LIVE_DAEMON_PASS / LOOPBACK_API_PASS / SYSTEM_TELEMETRY_PASS / CONNECTION_ENDPOINT_VISIBILITY_PASS / CROSS_USER_PROCESS_ATTRIBUTION_LIMITED`

Current source subject `0.1.0-0005`:

`UNPRIVILEGED_DESIGN_IMPLEMENTED / CAPABILITY_HELPER_SOURCE_IMPLEMENTED / NATIVE_UPGRADE_METADATA_IMPLEMENTED / PACKAGE_SOURCE_IMPLEMENTED / CI_PENDING / NOT INSTALLED / CAPABILITY_NOT_RUNTIME_QUALIFIED`

See `docs/DSM_7_2_2_RUNTIME_QUALIFICATION_20261004.md` for the live v0002/v0003 measurements and `archive/rejected-root-bridge-v0004/` for the superseded root-bridge attempt.

Source, build, package-source publication, Package Center discovery, installation, file-capability readback, daemon runtime, and behavioral qualification are separate evidence states.
