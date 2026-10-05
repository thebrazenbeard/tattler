# Windows Native Runtime Qualification — 2026-10-05

Status: SOURCE / BUILD / WORKLAPTOP RUNTIME QUALIFIED CANDIDATE; NOT MERGED OR RELEASED

## Exact subjects

- Repository: `thebrazenbeard/tattler`
- Base `main`: `9d92e987aa8217a395fdc41f317d0d43e50e371e`
- Implementation commit: `600e76fd4aafc493a67aed2220bb3b7022225196`
- Branch: `build/windows-native-v1`
- DSM source/package candidate on this branch: `0.2.0-0002`
- Native qualification host: Windows WorkLaptop
- Go toolchain used for live qualification: Go 1.27.0 windows/amd64

Source, build, runtime, CI, merge, and release are separate evidence states.

## Implemented Windows evidence

The Windows backend uses native Win32 evidence rather than WSL or Linux `/proc`:

- `GetExtendedTcpTable` for IPv4/IPv6 TCP endpoints and owning PID;
- listener rows from that same table for inbound/outbound classification;
- `QueryFullProcessImageNameW` for best-effort process executable/name enrichment;
- `GetSystemTimes` for host CPU utilization;
- `GlobalMemoryStatusEx` for physical-memory totals and availability.

The existing journal, API, findings pipeline, and dashboard remain shared with Linux/DSM.

## Verification

The exact implementation commit passed on WorkLaptop:

- `go test ./...`
- `go vet ./...`
- deterministic Windows AMD64 double build;
- Linux ARMv7 cross-build;
- `git diff --check`;
- native Windows loopback connection test proving endpoint recovery and owning PID;
- native listener-side direction test proving inbound classification;
- dashboard tests for embedded Tattler icon and unavailable-metric rendering.

Optimized executable:

- file: `dist/tattler-windows-amd64.exe`
- size: `7,299,072` bytes
- SHA-256: `bdf23f776917eca7f000d7ac42ca9f12ca92370d713cf934226ae1a4f6e9721d`

A second build from the exact implementation commit produced the same SHA-256.

## Live WorkLaptop readback

The exact executable above was started on WorkLaptop and queried through its loopback API.

Observed readback:

- collector: `windows-ip-helper`;
- platform: `windows`;
- current TCP connection count at readback: `112`;
- system sample count at readback: `3`;
- CPU cores: `12`;
- physical memory total: `33,270,576 KiB`;
- dashboard contains the Tattler brand icon;
- `/assets/tattler-icon.png` returned `image/png`;
- executable readback hash matched `bdf23f776917eca7f000d7ac42ca9f12ca92370d713cf934226ae1a4f6e9721d`.

The connection count and resource values are observations from that runtime cut, not fixed product properties.


## Linux compatibility probe on WorkLaptop

A Linux AMD64 build from the same source was executed under the WorkLaptop's WSL2 Linux kernel (`6.18.33.2-microsoft-standard-WSL2`). Tattler started with its Linux `proc-sampler`, served the loopback API, and returned Linux CPU, memory, disk, process, and connection evidence. This is a Linux runtime compatibility probe; it is not DSM hardware/runtime qualification and does not replace DS216 readback.

## Evidence ceiling

This first native Windows backend does **not** claim Windows UDP endpoint capture, ETW tracing, per-process CPU/I/O telemetry, native disk throughput/latency, Linux-style load average, Linux I/O wait, Linux swap activity, major-fault equivalence, or RAID state.

Linux-only metrics without a qualified Windows equivalent are emitted in `unavailable_metrics` and the dashboard renders applicable cards as `n/a` rather than silently presenting zero as a measurement.

Exact owning PID is supplied by the Windows endpoint table. Process name/executable enrichment is best-effort and may remain empty when Windows denies the process handle.

## CI / publication state

The branch adds a Windows GitHub Actions job that:

- runs Windows tests and vet;
- builds the Windows executable twice and requires identical SHA-256 output;
- performs a native loopback runtime smoke test;
- publishes `tattler-windows-amd64.exe` as a workflow artifact.

The Windows CI path passed on the published branch. The Linux package job initially failed its package-source provenance guard because the shared binary had changed while the DSM candidate still used the old `0.2.0-0001` package identity. The branch therefore advances the source/package candidate to `0.2.0-0002` instead of overwriting `0.2.0-0001` with different bytes. Final CI for `0.2.0-0002` remains a separate check.

No merge, release publication, installer creation, service installation, or protected deployment effect is established by this receipt.
