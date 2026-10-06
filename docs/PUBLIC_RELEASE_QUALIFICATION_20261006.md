# Tattler public-release qualification — 2026-10-06

## Exact subject

Source/package subject:

`thebrazenbeard/tattler@1c76ddae544a9bbb583b0a307f08d992699583ba`

Pull request:

`#2 — build/windows-native-v1`

GitHub Actions qualification:

- PR run `37533312343`: SUCCESS
- push run `37533307820`: SUCCESS

Both runs executed against the exact source/package subject above.

## Qualified build and test surfaces

The PR run completed both required jobs successfully.

`test-and-package`:

- root Go tests and vet;
- release-document and release-hygiene checks;
- deterministic ARMv7 binary double build;
- deterministic SPK double build;
- internal SPK verification;
- independent DSM 7.2.2 package verification using the pinned external verifier;
- package-source API tests;
- byte-for-byte package-source binding to the exact CI-built SPK.

`windows-agent`:

- native Windows collector tests and vet;
- deterministic Windows AMD64 agent double build;
- native loopback runtime smoke;
- Windows desktop companion tests and vet;
- Wails v2.14.0 Windows desktop build.

The successful PR run published three separate workflow artifacts:

- `tattler-armada38x-spk`;
- `tattler-windows-amd64`;
- `tattler-desktop-windows-amd64`.

## Package-source binding

The checked-in package-source manifest at this subject identifies:

- package: `Tattler`;
- version: `0.2.0-0002`;
- architecture: `armada38x`;
- DSM minimum build: `72806`;
- filename: `Tattler-armada38x-0.2.0-0002.spk`;
- size: `3,092,480` bytes;
- SHA-256: `d88c21f082daa6b56af2a70504204b79b6b49b028444f33b6ab37e4c7679bf41`.

The exact-head workflow's `Verify published package source binds exact SPK` step passed. This qualifies the source/package binding for the subject above.

## Public evidence semantics reviewed

The candidate documents and exposes three network observation kinds:

- `tcp_session`;
- `tcp_listener`;
- `udp_endpoint`.

The qualification does not promote sampled endpoint-table evidence into packet, kernel-lifecycle, DNS, QUIC, or remote-peer proof. A UDP endpoint is an endpoint observation only. Journal `open`/`close` events mean appearance/disappearance between samples.

The Windows desktop companion consumes the loopback API. It is not an independent telemetry collector.

## Evidence ceiling

This receipt qualifies source, tests, builds, workflow artifacts, and package-source provenance for the exact commit above.

It does **not** establish:

- that `0.2.0-0002` is installed on the live DS216;
- current DSM daemon behavior for this candidate;
- GitHub Release publication;
- Package Center upgrade of the live NAS;
- ETW/pcap/eBPF or packet-level telemetry;
- complete capture of short-lived network activity.

The live DS216 remains a separate runtime/install subject until an upgrade is explicitly authorized and read back.

Any source change after this exact subject requires its own CI readback before making an exact-head qualification claim.
