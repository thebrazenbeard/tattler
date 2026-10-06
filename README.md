# Tattler

Tattler is a low-overhead, observation-only host diagnostic agent built to answer a practical question:

> Why is this machine slow right now?

The primary deployment target is Synology DSM on DS216-class ARMv7 hardware. Tattler also has a native Windows collector and Windows AMD64 agent. Linux/DSM and Windows feed the same evidence model, bounded journal, loopback API, and dashboard.

Tattler is source-visible proprietary software, not open source. See [LICENSE](LICENSE), [COMMERCIAL_LICENSE.md](COMMERCIAL_LICENSE.md), [NOTICE](NOTICE), [CONTRIBUTING.md](CONTRIBUTING.md), and [SECURITY.md](SECURITY.md).

## Diagnostic surface

Tattler combines read-only system/process evidence with sampled network activity.

On Linux/DSM, system evidence comes from kernel `/proc` interfaces and includes CPU/load, I/O wait, memory and swap pressure, major faults, physical-disk rates/latency/queue evidence, Linux MD state, and bounded top-process evidence where readable.

Network observations use three explicit kinds:

- `tcp_session`: a sampled non-listening TCP row with local/remote endpoints and state;
- `tcp_listener`: a sampled local TCP listener; direction is `listen`;
- `udp_endpoint`: a sampled bound UDP endpoint with ownership evidence when available. A UDP endpoint does not prove that a datagram was sent or received and does not establish a remote DNS, QUIC, or other UDP peer.

On Linux these observations come from `/proc/net/{tcp,tcp6,udp,udp6}`. On Windows they come from IP Helper TCP/UDP OWNER_PID tables. Exact process attribution remains best-effort when the operating system cannot prove or expose it.

The journal persists versioned `open` / `close` observation events. Those names mean “appeared between samples” and “disappeared between samples”; they are not claims that Tattler observed a TCP SYN/FIN, a UDP datagram, or kernel lifecycle event.

The diagnosis layer emits bounded findings such as `memory-pressure`, `swap-churn`, `storage-wait`, `cpu-saturation`, `blocked-load`, and `major-faults`. Findings report supporting measurements rather than promoting sampled correlation into root-cause proof.

The loopback-only dashboard/API exposes:

- `/api/v1/status`
- `/api/v1/system`
- `/api/v1/findings`
- `/api/v1/current`
- `/api/v1/events`

## Native Windows agent

The Windows backend is native; it does not depend on WSL or Linux `/proc`.

Current Windows evidence uses:

- `GetExtendedTcpTable` for IPv4/IPv6 TCP sessions/listeners and owning PID;
- `GetExtendedUdpTable` OWNER_PID tables for IPv4/IPv6 UDP endpoints and owning PID;
- `QueryFullProcessImageNameW` for best-effort executable/name enrichment when the process handle is readable;
- `GetSystemTimes` for host CPU utilization;
- `GlobalMemoryStatusEx` for physical-memory totals and availability.

TCP listener evidence is retained as `tcp_listener` and also supports inbound/outbound classification of sampled `tcp_session` rows. UDP rows are `udp_endpoint` observations with no invented remote peer or direction.

Windows does not silently reinterpret Linux-only measurements. The API reports these as `unavailable_metrics` until a native equivalent is implemented and qualified: load average, Linux I/O wait, swap activity, major faults, disk throughput/latency, and RAID state. ETW, packet capture, per-process Windows CPU/I/O sampling, and native disk-latency telemetry are not claimed by this release.

Build and run the Windows agent:

```powershell
go test ./...
go vet ./...
New-Item -ItemType Directory -Force dist | Out-Null
go build -trimpath -buildvcs=false -ldflags="-s -w -buildid=" -o dist/tattler-windows-amd64.exe ./cmd/tattler

.\dist\tattler-windows-amd64.exe --state-dir .\tattler-state
# Dashboard: http://127.0.0.1:9147/
```

## Windows desktop companion

The separate `desktop/` Wails v2.14.0 module is a local companion, not a second telemetry collector. It consumes the agent's loopback API and shows host status, findings, and typed network observations.

At startup it attaches to an already-running loopback agent when available. If it launches a sibling Tattler agent itself, it tracks ownership and may stop only that child on shutdown; it does not terminate an externally started agent.

Build it on Windows:

```powershell
cd desktop
go test ./...
go vet ./...
go install github.com/wailsapp/wails/v2/cmd/wails@v2.14.0
& (Join-Path (go env GOPATH) "bin\wails.exe") build -clean -platform windows/amd64 -trimpath -webview2 browser
```

CI qualifies and publishes the Windows agent and desktop companion as separate workflow artifacts. The desktop artifact is a bundle containing both `tattler-desktop-windows-amd64.exe` and its required sibling `tattler-windows-amd64.exe`; extract and keep those two files together. The companion launches that sibling agent when no healthy loopback agent is already running. Building either artifact does not install or activate it.


## DSM privilege model

The active DSM package is deliberately ordinary:

```json
{
  "defaults": {
    "run-as": "package"
  },
  "username": "Tattler"
}
```

It requests no root lifecycle actions, no setuid executable, and no Linux file capabilities.

Two privilege experiments were rejected by live DSM 7.2.2 before installation:

- v0004: root lifecycle bootstrap helper;
- v0006: package-user helper carrying `cap_sys_ptrace`.

Synology documents file capabilities in `conf/privilege`, but this DS216's Package Center nevertheless classifies the capability-bearing v0006 SPK as a root-privileged package and refuses the unsigned package. Live installer behavior is therefore the governing compatibility evidence for this NAS.

## Unprivileged connection attribution

Linux `/proc/net/{tcp,tcp6,udp,udp6}` already exposes the numeric UID that owns each socket.

Tattler v0007 uses that evidence directly:

- retain exact PID/name/executable only when the package user can prove the socket inode through readable `/proc/<pid>/fd` links;
- always preserve the socket UID from the kernel table;
- resolve UID to account name from `/etc/passwd`;
- expose that account as `owner` when exact PID attribution is unavailable.

For DSM package services this is often still useful: a connection can be attributed to an account such as `PlexMediaServer` without falsely claiming which Plex worker owns the socket.

## V0.2 storage-evidence source candidate

Source candidate `0.2.0-0001` adds per-physical-disk evidence from Linux `/proc/diskstats` without adding privileges or hot-path SMART polling:

- per-device read/write throughput and IOPS;
- average I/O completion latency (`await_ms`);
- device busy time as `utilization_percent`;
- weighted average queue depth;
- richer `storage-wait` evidence naming the hottest observed physical disk;
- read-only Linux MD RAID state from `/proc/mdstat`, including array state/level, configured vs active members, health bitmap, and rebuild/resync progress when present;
- a bounded `raid-degraded` warning only when the MD member counts or health bitmap prove degradation.

Disk-rate fields are derived from counter deltas over the existing five-second system-sampling interval. MD RAID state is read directly from the kernel's `/proc/mdstat` surface. These are host-observation signals, not proof of a physical-disk failure.

The MD parser is source/test qualified against standard Linux MD formats. The attempted live DS216 SSH readback was inconclusive at the transport layer, so no claim is made yet about the NAS's current array membership or health.

The same source candidate also closes several journal-durability gaps without changing the privilege model:

- retained connection events are restored into `/api/v1/events` after daemon restart;
- startup reads newest journal generations first and stops once the bounded recent-event window is full;
- rotation errors are surfaced instead of discarded, with rollback/reopen behavior to preserve the live journal on failed rotation;
- final buffered-write failures are returned by `Close()` instead of being silently ignored;
- an interrupted rotation's preserved `.rotate-oldest` events remain visible to the recovery reader when newer retained generations do not fill the requested window.

A leftover `.rotate-oldest` still blocks later rotation rather than being guessed away automatically. That is intentional fail-closed behavior: the source preserves ambiguous recovery evidence instead of deleting it without enough state to prove the prior rotation completed.

`0.2.0-0001` was the prior SOURCE/BUILD/PACKAGE/EXACT-HEAD-CI-qualified candidate. The current branch advances the package candidate to `0.2.0-0002` because the shared binary changed for native Windows support. The live DS216 remains on runtime-qualified `0.1.0-0007` until a separate install/upgrade is explicitly authorized and read back.

## Native DSM updates

Tattler uses DSM's native Package Center upgrade path instead of a root self-updater.

The SPK declares:

- `silent_upgrade="yes"`
- `auto_upgrade_from="0.1.0-0003"`

The repository contains `package-source/`, a DSM package-source endpoint serving compatible `armada38x` hosts at DSM build `72806` or newer. The release manifest is derived from the finished SPK, and CI requires the published SPK to equal the deterministic CI artifact byte-for-byte.

The live DSM Package Source is already registered on the DS216 as `Tattler`.

## Performance posture

The DS216 has a very small resource budget. Network observation sampling defaults to 1 second, while the more expensive system/process scan defaults to 5 seconds. The root agent has no third-party Go dependencies; Wails dependencies are isolated to the optional `desktop/` module.

## Evidence ceiling

Tattler is a sampling diagnostic agent, not a packet sniffer or kernel tracing engine. Very short TCP sessions or endpoint changes can occur between samples. A retained UDP endpoint proves that the operating system exposed a bound endpoint during a sample; it does not prove datagram traffic, a remote peer, DNS activity, or QUIC activity.

Exact PID/process attribution can race process exit or be blocked by permissions. UID/account ownership is weaker than exact PID attribution and is reported separately. Linux namespaces and NAT can also limit what the host view proves. Windows IP Helper endpoint tables do not substitute for ETW or packet capture.

## Build and verification

```bash
go test ./...
go vet ./...

CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 \
  go build -trimpath -buildvcs=false -ldflags='-s -w -buildid=' \
  -o dist/tattler-linux-armv7 ./cmd/tattler

python tools/build_spk.py \
  --binary dist/tattler-linux-armv7 \
  --output dist/Tattler.spk

python tools/verify_spk.py dist/Tattler.spk
python -m unittest tools.release_hygiene_test tools.release_docs_test
python tools/release_hygiene.py --check .
```

CI pins Go 1.27.0 for the release candidate, builds the ARMv7 SPK deterministically, and also runs the independent strict DSM 7.2.2 verifier from `thebrazenbeard/spk-packager@89085efb9e439dfd05f26ad56d857ef71f2d52b1`.

## Status

Installed `0.1.0-0003` on DS216 / DSM 7.2.2:

`PACKAGE_CENTER_UPGRADE_PASS / LIVE_DAEMON_PASS / LOOPBACK_API_PASS / SYSTEM_TELEMETRY_PASS / CONNECTION_ENDPOINT_VISIBILITY_PASS / CROSS_USER_EXACT_PID_ATTRIBUTION_LIMITED`

v0006:

`SOURCE_BUILD_PASS / PACKAGE_SOURCE_DISCOVERY_PASS / DSM_INSTALL_REJECTED_ROOT_PRIVILEGE_CLASSIFICATION / NEVER_INSTALLED`

Current live runtime subject `0.1.0-0007`:

`PACKAGE_USER_ONLY / UID_OWNER_ATTRIBUTION_IMPLEMENTED / PACKAGE_SOURCE_LIVE / NATIVE_PACKAGE_CENTER_UPGRADE_PASS / LIVE_DAEMON_PASS / LOOPBACK_API_PASS / UID_OWNER_ATTRIBUTION_RUNTIME_PASS`

Current source/package candidate `0.2.0-0002`:

`TYPED_TCP_SESSION_LISTENER_UDP_ENDPOINT_SOURCE / WINDOWS_TCP_UDP_OWNER_PID_SOURCE / WINDOWS_DESKTOP_COMPANION_SOURCE / DISK_PRESSURE_EVIDENCE_IMPLEMENTED / JOURNAL_RECOVERY_HARDENED / RELEASE_HYGIENE_SOURCE / PACKAGE_SOURCE_BOUND_TO_CI_ARTIFACT / EXACT_HEAD_CI_RECEIPT_RECORDED / NOT_INSTALLED_ON_DSM / DSM_RUNTIME_NOT_QUALIFIED`

Exact source/package qualification is recorded in `docs/PUBLIC_RELEASE_QUALIFICATION_20261006.md`. Because CI qualification belongs to an exact commit, check the current PR head before treating later source changes as qualified.

Direct DSM readback after the native upgrade observed:
- installed version `0.1.0-0007`, architecture `armada38x`;
- daemon PID `8972`, running as DSM package user `Tattler`;
- approximately 8 MiB RSS and 1.4-1.5% CPU in the immediate readback;
- loopback API responsive;
- live connection owner labels including `http`, `tailscale`, and `WorkBridgeRelay` while exact `process` remained empty where cross-user FD proof was unavailable.

This confirms the intended evidence split: owner attribution is qualified from kernel socket UID/account mapping; exact cross-user PID attribution remains limited rather than inferred.

See `docs/DSM_7_2_2_RUNTIME_QUALIFICATION_20261004.md` for the live v0002/v0003 measurements and `docs/DSM_7_2_2_CAPABILITY_AND_NATIVE_UPDATES.md` for the rejected privilege experiments and current update architecture.

Source, build, package publication, Package Center discovery, installation, daemon runtime, and behavioral qualification are separate evidence states.
