# DSM 7.2.2 storage evidence qualification — 2026-10-04

Status: SOURCE-CANDIDATE QUALIFICATION; NOT INSTALLED

Subject:
- source base: `build/tattler-v1-sol-20261004@e588672da5a1f891439b04c32a1be14a47aa0871`
- candidate branch: `feature/tattler-v02-storage-evidence-20261004`
- live host class: DS216 / DSM 7.2.2
- live installed package remains `Tattler 0.1.0-0007`

## Live pre-change symptom

A fresh read-only Tattler API sample reported:
- I/O wait: 45.7%
- finding: `storage-wait`
- evidence exposed by v0007: only `I/O wait 45.7%`

The running v0007 package therefore detected the pressure but could not identify which physical block devices were carrying the backlog.

## Live kernel evidence

Read-only `/proc/diskstats` showed whole physical disks `sda` and `sdb`, plus partition rows, md RAID devices `md0` through `md3`, and device-mapper `dm-0`.

Read-only `/proc/mdstat` showed all four md arrays as RAID1 with `[2/2] [UU]` at observation time.

A five-second physical-disk counter delta produced the following candidate-derived evidence:

| device | read | write | busy | avg queue | avg await | I/O in progress at B |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| sda | 516,915 B/s | 165,069 B/s | 79.0% | 7.604 | 107.0 ms | 15 |
| sdb | 455,475 B/s | 165,069 B/s | 77.2% | 8.488 | 120.2 ms | 13 |

These values are derived from cumulative kernel counters over the five-second interval. They were not emitted by the installed v0007 binary.

## Interpretation ceiling

This evidence establishes substantial concurrent physical-disk activity and backlog during the sample. It does not establish that either physical disk is defective or singly responsible for host I/O wait.

On this host, RAID1, device-mapper, filesystem, controller, paging, and workload layers remain relevant. The candidate therefore reports up to two loaded physical disks as supporting evidence instead of declaring one disk the root cause.

## Candidate behavior

The source candidate reuses the existing five-second `/proc/diskstats` read and adds no new hot-path file polling. It exposes per-device throughput, busy percentage, average outstanding-I/O depth, average await, and in-progress I/O with source `proc-diskstats`.

Installation/runtime qualification of these new fields remains pending. The live NAS package is still v0007.
