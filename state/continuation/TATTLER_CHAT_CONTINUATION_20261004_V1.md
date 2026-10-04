# Tattler chat continuation — 2026-10-04 V1

Status: CANONICAL CHAT RECOVERY CHECKPOINT FOR DRAFT TATTLER WORK

Continuation command:

`TATTLER::RESTORE_AND_RUN::CHAT_CONTINUATION_20261004_V1`

## Recovery rule

A fresh chat receiving the continuation command should:

1. Treat the chat/session as a replaceable execution terminal.
2. Fetch `thebrazenbeard/tattler`.
3. Use branch `build/tattler-v1-sol-20261004` unless Patrick explicitly changes it.
4. Read this file first.
5. Re-read the exact current branch head and Draft PR #1 before editing because multiple chats/lane work can advance the branch.
6. Re-read the live DS216 over SSH before making runtime claims.
7. Do not merge PR #1. Patrick remains sole merge authority.
8. Preserve source/build/publication/install/runtime as separate evidence states.

## Canonical repository state at checkpoint preparation

Repository: `thebrazenbeard/tattler`

Draft PR: `#1 Build Tattler v0.1 diagnostic agent`

Branch: `build/tattler-v1-sol-20261004`

Source head immediately before this continuation checkpoint was added:

`92a07923a4ce4cdf7b034ea71c9e5b82b95e1b2c`

Exact-head GitHub Actions run:
- workflow `build`
- run `#36`
- conclusion `PASS`

Base/main observed:

`ab7d9894247f4d59d33cd9a394efead82126a585`

PR #1 remained open, draft, unmerged, and mergeable/clean at readback.

The branch may have advanced after this file was written. Always fetch and reconcile instead of assuming the hash above remains HEAD.

## Current live product subject

Current live package on DS216 / DSM 7.2.2:
- package `Tattler`
- version `0.1.0-0007`
- arch `armada38x`
- daemon user `Tattler`
- loopback API `127.0.0.1:9147`

Direct readback during checkpoint preparation observed:
- PID `8972`
- RSS approximately `9756 KiB`
- instantaneous CPU approximately `1.7-1.8%`
- API responsive
- sample timestamp `2026-10-04T20:44:47.444157841Z`
- load1 `2.76`
- CPU `13.4%`
- I/O wait `48.6%`
- available memory `313584 KiB`
- swap used `192348 KiB`
- major faults approximately `7.6/s`
- disk read approximately `1.54 MB/s`
- disk write approximately `0.34 MB/s`

Do not promote this single sample into a new long-duration performance baseline. The earlier v0002 ten-minute dataset remains the strongest duration-qualified overhead dataset.

## Current v0007 semantics

v0007 deliberately contains no privileged helper.

Current `spk/conf/privilege`:

```json
{
  "defaults": {
    "run-as": "package"
  },
  "username": "Tattler"
}
```

No root lifecycle actions. No setuid executable. No Linux file capabilities.

Connection attribution distinguishes:
- exact process attribution only when socket inode ownership is directly proven from readable `/proc/<pid>/fd` links;
- owner attribution from socket UID in `/proc/net/{tcp,tcp6,udp,udp6}`, mapped through `/etc/passwd`.

Runtime-qualified owner examples already observed:
- UID 1023 -> `owner: "http"`
- UID 165290 -> `owner: "tailscale"`
- UID 265891 -> `owner: "WorkBridgeRelay"`
- UID 0 -> `owner: "root"`

Where cross-user FD proof remains unavailable, `process` stays empty by design.

Current evidence ceiling:

`PACKAGE_USER_ONLY / EXACT_HEAD_CI_PASS / PACKAGE_SOURCE_LIVE / NATIVE_PACKAGE_CENTER_UPGRADE_PASS / LIVE_DAEMON_PASS / LOOPBACK_API_PASS / UID_OWNER_ATTRIBUTION_RUNTIME_PASS / CROSS_USER_EXACT_PID_ATTRIBUTION_LIMITED`
## Version history and lessons

### v0001

Installed package failed before daemon launch because:
- `precheckstartstop="yes"` was declared;
- `start-stop-status` did not implement DSM-required `prestart` / `prestop`;
- stopped `status` returned shell code 1 instead of DSM code 3.

### v0002

Corrected lifecycle contract.

Live qualification established:
- Package Center install PASS;
- daemon/package-user start PASS;
- loopback API PASS;
- system telemetry PASS;
- findings PASS;
- connection endpoint/direction visibility PASS;
- cross-user process attribution LIMITED.

Ten-minute overhead sample:
- Tattler CPU average 2.84%, p95 2.99%;
- Tattler RSS average 14.06 MiB;
- Tattler read rate average 0.14 MiB/s;
- only 2/120 samples exceeded 5% CPU.

During a real ugly stall:
- host CPU average 88.68%, p95 100%;
- load1 average 8.66 on 2 cores;
- iowait p95 40.16%, max 72.70%;
- Plex visible top-process CPU sum average 152.15%;
- `dm-0` and `sdb` reached 100% utilization;
- observed physical-disk read latency roughly 75-96 ms in one-second sampling and much worse longer averages;
- dominant Plex tasks included voice-activity detection, detection transcodes, and analysis.

Strongest explanation for that stall: concurrent Plex background detection/analysis saturated CPU and storage; paging/major-fault activity added pressure.

### v0003

Added real Tattler bird/gauge DSM package icons.
Installed successfully and ran the same qualified daemon bytes as v0002.

### v0004

Root-bridge experiment:
- tried root lifecycle bootstrap plus a tightly scoped package-management helper;
- DSM Package Center rejected the unsigned SPK because it requested root privileges.

Preserved only under:

`archive/rejected-root-bridge-v0004/`

Do not reactivate it without a new explicit decision.

### v0005

Intermediate native/capability packaging work.

The independent `spk-packager` strict DSM 7.2.2 verifier caught a missing `INFO checksum` before installation. v0005 was superseded rather than treated as a runtime failure.

### v0006

Kept package default `run-as: package`, moved cross-user process attribution to a tiny `tattler-procmap` helper with only `cap_sys_ptrace`.

It passed:
- Tattler verifier;
- independent strict `spk-packager` verifier;
- deterministic CI;
- publication binding;
- Package Source discovery.

DSM 7.2.2 still rejected installation as root-privileged.

v0006 was never installed.

Live DSM installer behavior supersedes the documentation-based assumption that this capability form would remain installable as an unsigned ordinary package on this DS216.

### v0007

Removed the capability helper entirely.
Added UID/account owner attribution.
Native package source discovered v0007.
Package Center upgraded v0003 -> v0007.
Runtime owner attribution passed.

## Canonical v0007 release/package evidence

Immutable release commit:

`427db77274bc877b2b7d04c4970210bcda237f9e`

Feed-pointer commit:

`4dda8de50e07673921b696fff11a9fc6774f7961`

Runtime-evidence branch head before this checkpoint:

`92a07923a4ce4cdf7b034ea71c9e5b82b95e1b2c`

Exact SPK:

`Tattler-armada38x-0.1.0-0007.spk`

SHA-256:

`a70428e2d9a0f132e5eb3b12d7c7c3608b12601200c023c51354083ea4b8b736`

MD5:

`d22eae80bd6b8226aaa168bdf403910a`

Size:

`2,273,280 bytes`

CI evidence:
- release-binding run #30 PASS;
- feed-pointer run #32 PASS;
- runtime-evidence run #34 PASS at earlier head `9bd2c543...`;
- current pre-checkpoint head run #36 PASS.

Independent package verifier remains pinned to:

`thebrazenbeard/spk-packager@89085efb9e439dfd05f26ad56d857ef71f2d52b1`

Keep it as an independent gate.
## Native package source

Live endpoint:

`https://fawkirqroyniueeqspif.supabase.co/functions/v1/tattler-package-source`

Supabase project:
- ID/ref `fawkirqroyniueeqspif`
- name `Tattler`
- organization ID `eryullybadeilkceiwod`
- region `us-east-2`
- status at checkpoint `ACTIVE_HEALTHY`

The project was repurposed from the previous MasaMune project because Patrick is limited to two Supabase projects.

Current Edge Functions observed:
- `tattler-package-source` — ACTIVE, version 3, `verify_jwt=false`
- historical `masamune-webhook` — ACTIVE, version 9, preserved for recoverability; do not delete merely because the project was repurposed.

The live package-source function is intentionally public/read-only and exposes no database mutation surface.

The DS216 already has this source registered as `Tattler`.

Do not waste time re-registering it unless readback proves it disappeared.

Package Source flow already proved:
- GET catalog PASS;
- POST catalog PASS;
- returned artifact hash PASS;
- DS216-side reachability PASS;
- Package Center discovery PASS;
- native upgrade PASS.

## Supabase/GitHub integration note

Patrick showed the Supabase dashboard after repurpose:
- project visibly named `Tattler`;
- GitHub repository integration selected `thebrazenbeard/tattler`;
- displayed production branch `main`;
- displayed working directory `.`.

The active Edge Function v3 was deployed directly during this work and is pinned to the immutable v0007 release commit through source-controlled `package-source/supabase/index.ts`.

Do not infer that merging PR #1 is authorized merely because the Supabase integration displays `main`.

## NAS access

NAS:
- DSM 7.2.2
- host/IP used in this work: `192.168.1.187`

SSH account:
- `psims85`

Working noninteractive key on Lappy:
- `C:\Users\patri\.ssh\ds216_lappy`

Known-good connection form:

```text
ssh -o BatchMode=yes -o IdentitiesOnly=yes -i C:\Users\patri\.ssh\ds216_lappy psims85@192.168.1.187 <read-only-command>
```

Important privilege boundary:
- key-authenticated SSH works as `psims85`;
- `sudo -n` does not currently grant root;
- `/usr/syno/bin/synowebapi` cannot be executed by this account without elevation;
- `/usr/syno/etc/packages/feeds` is `0600 root:root`.

Do not scrape or replay passwords/session secrets to cross that boundary.

It is currently unnecessary for Tattler updates because the native package source is already registered and working.

## Firefox / DSM UI Automation discovery

Blind foreground-coordinate automation previously misrouted text into another ChatGPT conversation. Do not use blind focus automation again.

A safer Windows UI Automation route was proven.

Temporary runtime dependency was installed only under:

`C:\Users\patri\AppData\Local\Temp\bt2-pywinauto`

The repo now contains the inspection helper:

`tools/experimental/dsm_package_center_uia_probe.py`

Targeting method:
- enumerate Firefox windows;
- require exact Firefox address-bar automation ID `urlbar-input`;
- require address value exactly `192.168.1.187:5001`;
- then interact by accessible control name/automation identity.

Observed DSM accessibility tree included:
- Firefox tab `The Sims Vault`;
- address `192.168.1.187:5001`;
- DSM desktop `Package Center`;
- Package Center `Manual Install`;
- Package Center `Settings`;
- installed `Tattler`;
- stale error dialog from the rejected privilege build.

This proves UIA is viable if future DSM GUI automation is needed, but native/package APIs should remain preferred.

## Accidental inter-chat relay discovery

During a failed terminal-focus automation attempt, a harmless diagnostic string intended for the NAS SSH terminal was submitted into a different open ChatGPT conversation instead.

That proved a workstation-mediated inter-chat text transport is possible, but brittle.

This was reported to One on the canonical Chat Communication Bus with precise implementation/qualification instructions.

Bus messages from Two:
- `two-one-20261004-workstation-interchat-relay-6f1e`
- `two-one-20261004-workstation-interchat-relay-implementation-v1`

Latest Two-lane commit after the detailed directive:
- `3976dff44c2fdfa9e5d70285032fb9ba216e7cd7`

Do not treat workstation relay as canonical transport. Bus remains canonical.

## Current source layout / useful files

Primary docs:
- `README.md`
- `docs/ARCHITECTURE.md`
- `docs/ROADMAP.md`
- `docs/RESEARCH.md`
- `docs/DSM_7_2_2_PACKAGE_CONTRACT.md`
- `docs/DSM_7_2_2_CAPABILITY_AND_NATIVE_UPDATES.md`
- `docs/DSM_7_2_2_RUNTIME_QUALIFICATION_20261004.md`

Packaging:
- `spk/INFO`
- `spk/conf/privilege`
- `spk/scripts/start-stop-status`
- `tools/build_spk.py`
- `tools/verify_spk.py`
- `tools/update_package_source.py`

Package source:
- `package-source/release.json`
- `package-source/api/catalog.js`
- `package-source/supabase/index.ts`
- `package-source/public/releases/Tattler-armada38x-0.1.0-0007.spk`

Runtime attribution:
- `internal/uidmap/uidmap.go`
- connection model/API code under `internal/model`, `internal/procnet`, `internal/server`, and `cmd/tattler`

Historical rejected privileged design:
- `archive/rejected-root-bridge-v0004/`
## Recommended next work

Do not reopen already-solved package-source registration or v0006 capability work.

Recommended execution order:

### 1. Re-establish fresh exact subject

At start of a new chat:
- fetch branch;
- inspect Draft PR #1 head;
- read exact-head CI status;
- read installed Tattler INFO from the NAS;
- read live `/api/v1/status` and `/api/v1/current`;
- verify the Supabase project/function is still active.

### 2. Long-duration v0007 overhead qualification

The strongest long-duration overhead data is still v0002.

Run a new 10-15 minute v0007 sample under:
- ordinary idle/light conditions;
- one naturally occurring Plex/background-analysis interval if available.

Compare:
- Tattler CPU avg/p95/max;
- RSS avg/p95/max;
- Tattler I/O;
- host CPU/load/iowait;
- fault/swap rates;
- owner-attribution behavior.

Do not manufacture a stall.

### 3. Close V0.1 qualification gaps

Re-test/record:
- restart persistence of connection journal;
- rotation behavior without data loss;
- controlled inbound/outbound TCP evidence if not already durably captured;
- alert/finding churn under normal host use.

If these pass, update `docs/ROADMAP.md` to mark V0.1 runtime-ready for this exact DS216/DSM subject.

### 4. Begin V0.2 storage/DSM evidence

This is the highest-value next feature because the real Plex stall exposed storage queueing that current Tattler only partially explains.

Add optional low-frequency evidence for:
- DSM/md RAID state where readable;
- device/volume identity;
- block latency/queue depth where kernel interfaces permit;
- SMART only at a very low cadence and only where safely/readably available;
- explicit source/provenance fields.

Do not put expensive SMART polling in the hot path.

Prefer evidence that can explain:
- 100% block-device utilization;
- high await;
- queue buildup;
- RAID/volume degradation;
while keeping `iowait` correctly framed as evidence rather than proof of disk failure.

### 5. Then V0.3 durable diagnostic history

Persist compact/downsampled system history and finding transitions so the operator can inspect the minutes preceding a stall without writing every five-second raw sample.

## Roadmap after V0.3

V0.4:
- higher-fidelity connection collector interface;
- Netfilter conntrack where kernel/privilege permits;
- preserve V1 event envelope;
- expose loss/overflow counters;
- evaluate pcap/eBPF only on capable hosts.

V0.5:
- richer correlation of CPU/RSS/I/O, connection activity, storage wait, and finding windows;
- UID/user mapping is already partially pulled forward by v0007;
- optional DNS correlation with raw-vs-derived provenance.

V0.6:
- authenticated multi-host export/aggregation as a separate optional surface;
- local-only operation must remain valid.
## Do-not-do list

- Do not merge PR #1 without Patrick.
- Do not reintroduce the v0004 privileged lifecycle design by default.
- Do not reintroduce the v0006 capability-helper design on this NAS; live DSM rejected it.
- Do not infer exact PID from UID owner.
- Do not expose Tattler HTTP publicly; loopback-only is the current safety boundary.
- Do not let Supabase publication outrun source-controlled immutable release binding.
- Do not remove the preserved `masamune-webhook` merely because the Supabase project is now named Tattler.
- Do not use blind Firefox focus/coordinate automation.
- Do not claim a current runtime result without fresh readback.

## Lantern/currentness note

This continuation request materially involved recovery/currentness, so the Project Lantern V3 handshake was consulted.

The required exact WoWSQL target is:

`bt2-479e4ad9`

In this session the available Aiven project listing did not expose that exact target, so the required Lantern projection preflight -> B0 -> payload -> B1 sequence could not be executed against the required target.

Per Project instructions:
- Lantern currentness was **not established**;
- no Supabase or other provider was used as a Lantern fallback;
- no claim in this checkpoint depends on Lantern material currentness.

The checkpoint instead binds concrete claims to current GitHub source, exact CI evidence, live DS216 readback, and live Supabase package-source readback.

## Fresh-chat continuation command behavior

When Patrick sends:

`TATTLER::RESTORE_AND_RUN::CHAT_CONTINUATION_20261004_V1`

the new chat should:
- fetch current Git source first;
- read this checkpoint;
- compare branch HEAD to the hashes above;
- absorb newer source if the branch advanced;
- read live NAS state;
- resume the recommended next work without asking Patrick to restate this history;
- keep PR #1 draft/unmerged unless Patrick explicitly changes that instruction.
