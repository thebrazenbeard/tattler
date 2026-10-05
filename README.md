# Tattler

Tattler is a low-overhead host diagnostic agent built to answer a practical question:

> Why is this machine slow right now?

The first deployment target is Synology DSM on DS216-class ARMv7 hardware. Tattler now also has a native Windows collector and Windows AMD64 executable path; the Linux/DSM and Windows backends feed the same evidence model, journal, API, and dashboard.

## V0.1 diagnostic surface

Tattler combines two read-only evidence streams:

- system/process telemetry from Linux `/proc`: CPU use, load, I/O wait, memory and swap pressure, major faults, disk throughput, and top processes by CPU/RSS/I/O;
- host connection telemetry from `/proc/net/tcp`, `tcp6`, `udp`, and `udp6`, with best-effort exact PID attribution plus unprivileged socket-owner attribution.

The diagnosis layer emits bounded findings such as `memory-pressure`, `swap-churn`, `storage-wait`, `cpu-saturation`, `blocked-load`, and `major-faults`. Findings report the supporting measurement rather than pretending to establish a root cause the evidence cannot prove.

Connection changes are persisted as versioned `open` / `close` JSONL events with host, observation time, collector source, and deterministic event IDs.

The loopback-only dashboard/API exposes:

- `/api/v1/status`
- `/api/v1/system`
- `/api/v1/findings`
- `/api/v1/current`
- `/api/v1/events`


## Native Windows agent

The Windows backend is native; it does not depend on WSL or Linux `/proc`.

Current Windows evidence uses:

- `GetExtendedTcpTable` for IPv4/IPv6 TCP endpoint state and owning PID;
- `QueryFullProcessImageNameW` for best-effort executable/name enrichment when the process handle is readable;
- `GetSystemTimes` for host CPU utilization;
- `GlobalMemoryStatusEx` for physical-memory totals and availability.

Listener evidence from the same Windows TCP table is used to classify established sockets as inbound or outbound. Exact PID comes from the Windows endpoint table; process name/executable is left empty when Windows does not permit that enrichment.

Windows does not silently reinterpret Linux-only measurements. The API reports these as `unavailable_metrics` until a native equivalent is implemented and qualified: load average, Linux I/O wait, swap activity, major faults, disk throughput/latency, and RAID state.

Windows UDP endpoint capture, ETW tracing, per-process CPU/I/O sampling, and native disk-latency telemetry are not claimed by this first Windows backend. Those remain separate follow-on evidence surfaces rather than being inferred from TCP or host-level counters.

Build and run on Windows:

```powershell
go test ./...
go vet ./...
New-Item -ItemType Directory -Force dist | Out-Null
go build -trimpath -buildvcs=false -ldflags="-s -w -buildid=" -o dist/tattler-windows-amd64.exe ./cmd/tattler

.\dist\tattler-windows-amd64.exe --state-dir .\tattler-state
# Dashboard: http://127.0.0.1:9147/
```

CI builds the Windows binary twice, requires byte-identical SHA-256 output, performs a native loopback API smoke test, and publishes `tattler-windows-amd64.exe` as a workflow artifact.

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

`0.2.0-0001` is currently **SOURCE/BUILD/PACKAGE/EXACT-HEAD-CI VERIFIED ONLY**. The live DS216 remains on runtime-qualified `0.1.0-0007` until a separate install/upgrade is explicitly authorized and read back.

## Native DSM updates

Tattler uses DSM's native Package Center upgrade path instead of a root self-updater.

The SPK declares:

- `silent_upgrade="yes"`
- `auto_upgrade_from="0.1.0-0003"`

The repository contains `package-source/`, a DSM package-source endpoint serving compatible `armada38x` hosts at DSM build `72806` or newer. The release manifest is derived from the finished SPK, and CI requires the published SPK to equal the deterministic CI artifact byte-for-byte.

The live DSM Package Source is already registered on the DS216 as `Tattler`.

## Performance posture

The DS216 has a very small resource budget. Connection sampling defaults to 1 second, while the more expensive system/process scan defaults to 5 seconds. The runtime has no third-party Go dependencies.

## Evidence ceiling

V0.1 is a sampling diagnostic agent, not a packet sniffer or kernel tracing engine. Very short connections can occur between samples. Unconnected inbound UDP is not represented as a connection. Exact PID attribution can race process exit or be blocked by permissions. UID/account ownership is weaker than exact PID attribution and is reported separately.

Linux namespaces and NAT can also limit what the host view proves.

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
```

CI also runs the independent strict DSM 7.2.2 verifier from `thebrazenbeard/spk-packager@89085efb9e439dfd05f26ad56d857ef71f2d52b1`.

## Status

Installed `0.1.0-0003` on DS216 / DSM 7.2.2:

`PACKAGE_CENTER_UPGRADE_PASS / LIVE_DAEMON_PASS / LOOPBACK_API_PASS / SYSTEM_TELEMETRY_PASS / CONNECTION_ENDPOINT_VISIBILITY_PASS / CROSS_USER_EXACT_PID_ATTRIBUTION_LIMITED`

v0006:

`SOURCE_BUILD_PASS / PACKAGE_SOURCE_DISCOVERY_PASS / DSM_INSTALL_REJECTED_ROOT_PRIVILEGE_CLASSIFICATION / NEVER_INSTALLED`

Current live runtime subject `0.1.0-0007`:

`PACKAGE_USER_ONLY / UID_OWNER_ATTRIBUTION_IMPLEMENTED / PACKAGE_SOURCE_LIVE / NATIVE_PACKAGE_CENTER_UPGRADE_PASS / LIVE_DAEMON_PASS / LOOPBACK_API_PASS / UID_OWNER_ATTRIBUTION_RUNTIME_PASS`

Current source/package candidate `0.2.0-0001`:

`DISK_PRESSURE_EVIDENCE_IMPLEMENTED / RESTART_EVENT_RESTORE_IMPLEMENTED / BOUNDED_HISTORY_SCAN / ROTATION_FAILURE_SAFE / CLOSE_FLUSH_ERRORS_SURFACED / RECOVERY_FILE_VISIBLE / LOCAL_TEST_VET_PASS / DETERMINISTIC_ARMV7_BUILD_PASS / DETERMINISTIC_SPK_PASS / INDEPENDENT_DSM_7_2_2_VERIFY_PASS / EXACT_HEAD_CI_PASS / CI_PACKAGE_SOURCE_BOUND / NOT_INSTALLED / RUNTIME_NOT_QUALIFIED`

Direct DSM readback after the native upgrade observed:
- installed version `0.1.0-0007`, architecture `armada38x`;
- daemon PID `8972`, running as DSM package user `Tattler`;
- approximately 8 MiB RSS and 1.4-1.5% CPU in the immediate readback;
- loopback API responsive;
- live connection owner labels including `http`, `tailscale`, and `WorkBridgeRelay` while exact `process` remained empty where cross-user FD proof was unavailable.

This confirms the intended evidence split: owner attribution is qualified from kernel socket UID/account mapping; exact cross-user PID attribution remains limited rather than inferred.

See `docs/DSM_7_2_2_RUNTIME_QUALIFICATION_20261004.md` for the live v0002/v0003 measurements and `docs/DSM_7_2_2_CAPABILITY_AND_NATIVE_UPDATES.md` for the rejected privilege experiments and current update architecture.

Source, build, package publication, Package Center discovery, installation, daemon runtime, and behavioral qualification are separate evidence states.
