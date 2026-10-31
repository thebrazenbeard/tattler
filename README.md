# Tattler — sampled host diagnostics

**A lightweight, observation-only diagnostic agent for Linux, Synology DSM, and Windows.**

Tattler helps operators investigate questions such as “Is this host under memory pressure?”, “Is storage waiting on I/O?”, and “Which TCP sessions or UDP endpoints are visible right now?” It collects bounded system and network observations, reports evidence-backed findings, stores a rotated event journal, and serves a local dashboard and JSON API. It does **not** change network settings, terminate processes, capture packet payloads, or prove the root cause of a slowdown.

The core agent is written in Go. A separate optional [Windows desktop companion](desktop/) uses Wails and reads the agent's API; it is not another collector.

## What is implemented

| Capability | Linux / DSM | Windows |
| --- | --- | --- |
| System sampling | CPU, load, I/O wait, memory/swap, major faults, physical-disk and Linux MD evidence, bounded process summaries | Host CPU and physical-memory measurements; unsupported Linux metrics explicitly marked unavailable |
| Network sampling | `/proc/net/{tcp,tcp6,udp,udp6}` | Native IP Helper TCP/UDP OWNER_PID tables |
| Observation kinds | `tcp_session`, `tcp_listener`, `udp_endpoint` | Same typed model |
| Ownership | Kernel socket UID/account; exact PID/process when accessible | Owning PID and best-effort process-name enrichment |
| Output | Evidence-bearing findings, bounded current observations and sampled event history | Same journal, API, and dashboard |

Network endpoint samples are **not** packet traces. Journal `open`/`close` means an observation appeared/disappeared between samples, not that Tattler saw a TCP handshake or disconnect. A UDP endpoint does not prove datagram traffic or a remote peer. Process attribution can be limited by races or OS permissions.

For the detailed evidence pipeline and limitations, see [Architecture](docs/ARCHITECTURE.md).

## Quick start

Prerequisites: **Go 1.23 or newer** and an accessible Linux `/proc` filesystem or a supported Windows environment. Build and run from a checkout:

```sh
git clone https://github.com/thebrazenbeard/tattler.git
cd tattler
go test ./...
go run ./cmd/tattler --state-dir ./tattler-state
```

On Windows, the same Go package uses native collectors rather than WSL. For a standalone executable:

```powershell
go test ./...
go build -o tattler.exe ./cmd/tattler
.\tattler.exe --state-dir .\tattler-state
```

Then open **http://127.0.0.1:9147/** on the host running the agent, or query the local API:

```sh
curl http://127.0.0.1:9147/api/v1/status
curl http://127.0.0.1:9147/api/v1/findings
curl 'http://127.0.0.1:9147/api/v1/events?limit=100'
```

The agent refuses non-loopback binds. Its HTTP service is local-only, **not an authenticated authorization service**. Keep its state and diagnostic information within your intended host trust boundary; only run it where you are authorized to observe process and network metadata.

The default network sampling interval is one second and system sampling is five seconds. `--poll`, `--metrics-interval`, `--state-dir`, and the bounded journal flags can be inspected with the compiled agent's `-h` option. Sampling can miss short-lived activity.

## JSON API

All endpoints are served on the local loopback interface:

| Route | Result |
| --- | --- |
| `/api/v1/status` | Collector, uptime, sample counts, latest system evidence |
| `/api/v1/system` | Bounded recent system samples (`?limit=` supported) |
| `/api/v1/findings` | Current measurement-backed diagnostic findings |
| `/api/v1/current` | Currently sampled typed endpoint observations and observed age |
| `/api/v1/events` | Bounded retained appearance/disappearance events (`?limit=` supported) |

The recent-event journal persists across restarts and supports bounded rotation/recovery. Reported findings are heuristic diagnoses supported by sampled measurements, not proof of a faulty disk, malicious traffic, or a specific process's intent.

## Windows desktop companion

The optional [Wails desktop module](desktop/) presents the loopback agent's status, findings, and endpoint table. It first attempts to attach to an existing healthy agent. If it launches its bundled sibling agent, it may stop only that child when closing; it does not take ownership of an externally started process.

Desktop builds use the separate `desktop/go.mod` and Wails v2 tooling. The [build workflow](.github/workflows/build.yml) tests and packages the native Windows agent and companion separately. A desktop bundle needs its sibling Tattler agent executable alongside it; building a bundle is not installation or runtime qualification.

## Synology DSM packages

The source includes DSM [SPK metadata and lifecycle scripts](spk/), a deterministic [package builder/verifier](tools/build_spk.py), and a [package-source catalog](package-source/). The package design uses a dedicated package user rather than root; the agent's UI/API remains loopback-only.

Source and package artifacts are organized for the generic DSM families **x86_64**, **ARMv7**, and **ARMv8/AArch64**. Platform mappings, release provenance, and upgrade details are documented in [DSM package contract](docs/DSM_7_2_2_PACKAGE_CONTRACT.md) and [multi-architecture qualification](docs/PUBLIC_RELEASE_QUALIFICATION_20261006_V0004.md).

Source build, CI package verification, package-source publication, installation, and **live runtime verification** are separate gates. A CI-produced SPK or a tested route does not establish that a particular NAS accepted or installed it. Historical device-specific observations belong in [qualification evidence](docs/DSM_7_2_2_RUNTIME_QUALIFICATION_20261004.md), not generic installation requirements.

## Current source/package candidate

The repository's recorded `0.2.0-0004` source/package qualification identifies `MULTIARCH_X86_64_ARMV7_ARMV8_SOURCE`, `PACKAGE_SOURCE_BOUND_TO_CI_ARTIFACT`, and `EXACT_HEAD_CI_RECEIPT_RECORDED`. These are **source and CI artifact labels**, not an installation or live-host acceptance claim. Consult [the exact qualification receipt](docs/PUBLIC_RELEASE_QUALIFICATION_20261006_V0004.md) (`docs/PUBLIC_RELEASE_QUALIFICATION_20261006_V0004.md`) before relying on any historical package candidate.

## Development and verification

The main Go module declares Go 1.23. The repository's CI currently uses Go 1.27, tests Linux and native Windows, builds platform-specific agents, checks deterministic SPK construction and package-source binding, and runs bounded Python/Node regression checks.

```sh
go test ./...
go vet ./...
python3 -m unittest tools.release_hygiene_test tools.release_docs_test tools.spk_lifecycle_test tools.spk_multiarch_test tools.editor_task_guard_test
node package-source/test.js
python3 tools/release_hygiene.py --check .
```

Some platform and packaging checks require the applicable toolchains or GitHub Actions runners; a local unit-test pass is not a DSM install test. See the [CI workflow](.github/workflows/build.yml) and [release evidence](docs/PUBLIC_RELEASE_QUALIFICATION_20261006_V0004.md).

## License and security

Tattler is **source-visible proprietary software, not open source**. The [license](LICENSE) permits defined private noncommercial evaluation but does not grant general redistribution, commercial use, or hosted deployment rights. See [commercial licensing](COMMERCIAL_LICENSE.md), [third-party notices](NOTICE), [contribution requirements](CONTRIBUTING.md), and [security reporting](SECURITY.md).

Tattler is a diagnostic observer. It does not provide provider identity attestation, consent management, immutable host-owned authority bindings, or an authorization service for other applications.
