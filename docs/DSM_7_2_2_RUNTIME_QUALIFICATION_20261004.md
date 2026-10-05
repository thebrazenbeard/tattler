# Tattler DSM 7.2.2 live runtime qualification — 2026-10-04

## Subject

Installed package:

- package: `Tattler`
- version: `0.1.0-0002`
- DSM minimum declared: `7.2-72806`
- architecture: `armada38x`
- installed target: `/volume1/@appstore/Tattler`
- package var: `/volume1/@appdata/Tattler`
- live daemon PID observed: `3724`
- live package UID/GID observed: `176516/176516`

This qualification applies to installed `0.1.0-0002`, not to the icon-only `0.1.0-0003` source artifact.

## Runtime state

The daemon was observed running as the DSM package identity, not as root:

```
/volume1/@appstore/Tattler/bin/tattler \
  --listen 127.0.0.1:9147 \
  --state-dir /volume1/@appdata/Tattler/state \
  --poll 1s \
  --metrics-interval 5s
```

Observed `/proc/3724/status`:

- `Uid: 176516 176516 176516 176516`
- `Gid: 176516 176516 176516 176516`
- `VmRSS`: approximately 17.2 MiB in the final readback
- threads: 9

Tattler's loopback API remained responsive during severe NAS load.

## Ten-minute overhead sample

120 system samples spanning approximately 10 minutes:

- Tattler CPU: average 2.84%, p95 2.99%, max 28.29%
- Tattler RSS: average 14.06 MiB, p95 16.29 MiB, max 17.07 MiB
- Tattler read rate: average 0.14 MiB/s, p95 0.43 MiB/s, max 0.79 MiB/s
- samples with Tattler CPU >5%: 2 / 120

The two CPU outliers were approximately 28.29% and 24.24%. The remaining distribution stayed near 3% or below. These spikes should be treated as an optimization target, but they do not explain the observed host-wide slowdown.

## Host stall captured by Tattler

Across the same 120 samples:

- host CPU average: 88.68%
- host CPU p95: 100%
- load1 average: 8.66 on 2 cores
- load1 max: 10.25
- I/O wait average: 9.63%
- I/O wait p95: 40.16%
- I/O wait max: 72.70%
- disk read average: 7.65 MiB/s
- disk read max: 17.75 MiB/s
- major faults average: 16.68/s
- major faults max: 46.40/s
- available memory minimum: 129.29 MiB
- swap used: approximately 359–362 MiB
- visible Plex top-process CPU sum average: 152.15%

A representative Tattler sample showed:

- CPU 99.2%
- load1 10.18
- swap-in 42.2 pages/s
- major faults 38.2/s
- disk reads about 18.6 MB/s
- three active findings: `swap-churn`, `cpu-saturation`, `major-faults`

## Independent shell corroboration

A direct `iostat -x 1 2` snapshot during the stall corroborated Tattler:

- `dm-0` reached 100% utilization
- `sdb` reached 100% utilization
- one-second read await was roughly 75–96 ms on the physical disks
- longer-average read await was roughly 119–156 ms on the physical disks
- `dm-0` longer-average read await was roughly 180 ms
- queue depth was materially elevated

This independently supports storage contention / queueing as part of the observed slowdown.

## Plex activity observed during the stall

The high-load period was dominated by Plex background analysis rather than ordinary playback.

Examples observed:

- `Plex Media Scanner --voice-activity-detection` at roughly 78% CPU
- two concurrent `Plex Transcoder` detection jobs at roughly 83% and 67% CPU
- two concurrent `Plex Media Scanner --analyze --no-thumbs` jobs at roughly 8% CPU each

The detection transcodes were operating against media under `TV Shows/Archer` and writing into Plex's `Cache/Transcode/Detection` tree.

The strongest current explanation for the "slow and shitty" interval is therefore concurrent Plex background detection/analysis saturating both CPU and the storage stack, with paging/major-fault activity adding pressure.

## /proc permission ceiling

The package account can read enough host `/proc` state to produce:

- aggregate CPU/load/memory/swap/disk metrics
- process PID/name/RSS/CPU summaries across other processes
- socket tables and connection direction/endpoints

However, connection records showed empty `process` attribution objects even when socket inodes and UIDs were visible. This is consistent with the unprivileged DSM package account being unable to inspect other processes' `/proc/<pid>/fd` links.

Per-process I/O for other package users also commonly remained zero while Tattler's own per-process I/O was visible, consistent with additional `/proc/<pid>/io` permission limits.

Therefore:

- host/process pressure visibility: qualified
- socket endpoint visibility: qualified
- cross-user socket-to-process attribution: not qualified under the current unprivileged package identity
- cross-user per-process I/O attribution: incomplete

This limitation is preferable to silently elevating the daemon to root.

## DSM package-control observation

`/usr/syno/bin/synopkg status Tattler` did not return within an 8-second timeout during the severe load period, despite the Tattler daemon and loopback API remaining responsive.

This suggests DSM package-control responsiveness can degrade independently of the package process itself. It is not evidence that Tattler was unhealthy.

## Qualification result

Exact subject `Tattler 0.1.0-0002`:

- Package Center installation: PASS
- DSM 7.2.2 lifecycle start: PASS
- live daemon: PASS
- package-user execution: PASS
- loopback API responsiveness under heavy load: PASS
- system telemetry: PASS
- evidence-bearing findings: PASS
- TCP endpoint/direction visibility: PASS
- steady-state overhead target: PASS with optimization note
- cross-user socket-to-process attribution: LIMITED
- live icon-bearing v0003 runtime: NOT TESTED

No merge, provider change, privilege escalation, or package-user elevation is implied by this result.


## v0003 live upgrade addendum

Package Center subsequently upgraded the DS216 from `0.1.0-0002` to `0.1.0-0003`.

Direct SSH readback after the upgrade showed:

- `package="Tattler"`
- `version="0.1.0-0003"`
- `arch="armada38x"`
- daemon PID observed: `31934`
- daemon user: `Tattler`
- daemon command line unchanged
- loopback API responsive
- daemon binary intentionally byte-identical to the previously qualified v0002 binary

Representative immediate post-upgrade readings included approximately 7 MiB RSS / 2.3% CPU for Tattler while the host itself remained heavily loaded by Plex and storage wait.

This addendum establishes the v0003 package lifecycle/icon upgrade and daemon restart. It does not change the v0002 ten-minute overhead dataset's exact subject; those measurements remain bound to v0002's identical daemon binary.

The two DSM icon assets are part of the v0003 SPK archive. Their absence under `/var/packages/Tattler/target/..` after installation is not evidence that Package Center rejected them; DSM consumes package metadata/assets during installation rather than preserving the original outer SPK layout there.


## v0007 native-update/runtime addendum

DSM Package Center discovered and upgraded Tattler through the registered native package source to `0.1.0-0007`.

Exact source/runtime subject:
- source head observed before runtime qualification: `9bd2c543b66e766bf3098b269b8461fd66f9c508`
- exact-head GitHub Actions run #34: PASS
- canonical v0007 SPK SHA-256: `a70428e2d9a0f132e5eb3b12d7c7c3608b12601200c023c51354083ea4b8b736`

Direct SSH readback after installation showed:
- `package="Tattler"`
- `version="0.1.0-0007"`
- `arch="armada38x"`
- daemon PID `8972`
- daemon user `Tattler`
- command line:
  `/volume1/@appstore/Tattler/bin/tattler --listen 127.0.0.1:9147 --state-dir /volume1/@appdata/Tattler/state --poll 1s --metrics-interval 5s`
- immediate daemon RSS approximately 8 MiB
- immediate daemon CPU approximately 1.4-1.5%
- loopback `/api/v1/status` responsive

A first poll immediately after Package Center reported the new INFO version observed no running process/API yet. Package lifecycle logs then showed DSM's v0007 `start-stop-status start` returning success, and subsequent direct readback observed the live PID/API. The early gap is therefore treated as upgrade/start transition timing, not a persistent crash.

### v0007 owner-attribution behavior

Live `/api/v1/current` records demonstrated the intended unprivileged UID/account mapping:
- UID 1023 -> `owner: "http"` for DSM web connections;
- UID 265891 -> `owner: "WorkBridgeRelay"`;
- UID 165290 -> `owner: "tailscale"`;
- root-owned sockets reported `owner: "root"`.

For these cross-user sockets the `process` object remained empty where Tattler could not prove the owning PID through readable `/proc/<pid>/fd` links.

This is the intended evidence model:
- exact PID/name/executable attribution: only when directly proven from readable FD links;
- account-owner attribution: kernel socket UID mapped through `/etc/passwd`;
- no PID is inferred from UID ownership.

### v0007 qualification result

Exact subject `Tattler 0.1.0-0007`:
- native Package Source discovery: PASS
- Package Center upgrade: PASS
- unsigned package privilege compatibility: PASS
- package-user execution: PASS
- live daemon: PASS
- loopback API: PASS
- system telemetry: PASS
- connection endpoint/direction visibility: PASS
- UID/account owner attribution: PASS
- exact cross-user PID attribution: LIMITED by DSM `/proc` permissions
- immediate resource overhead: PASS (approximately 8 MiB RSS, 1.4-1.5% CPU in the observed readback)

The prior v0002 ten-minute overhead dataset remains the stronger long-duration overhead measurement. The v0007 values above are an immediate post-upgrade readback, not a replacement ten-minute dataset.
