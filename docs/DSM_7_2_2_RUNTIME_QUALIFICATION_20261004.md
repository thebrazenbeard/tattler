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
