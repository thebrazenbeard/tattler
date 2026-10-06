# Tattler DSM SPK 0.2.0-0004 multi-architecture qualification — 2026-10-06

## Exact subject

Source/package subject:

`thebrazenbeard/tattler@02fd6cca78d8f16465856bcbea3cafbde977e239`

Stacked pull request:

`#5 — improve/spk-multiarch-v0004-20261006`

GitHub Actions qualification:

- PR run `37544117888`: SUCCESS
- push run `37544111855`: SUCCESS

Both runs executed against the exact subject above.

## Package family

Package version: `0.2.0-0004`

DSM minimum build: `72806`

Published architecture artifacts:

- `Tattler-x86_64-0.2.0-0004.spk`
  - generic package arch: `x86_64`
  - ELF e_machine: `62`
  - size: `3,184,640` bytes
  - SHA-256: `08b0f3cba03eadb87ca7a996ad87c0eeb066641df1b8ec81699d9b74db54e515`
- `Tattler-armv7-0.2.0-0004.spk`
  - generic package arch: `armv7`
  - ELF e_machine: `40`
  - size: `3,092,480` bytes
  - SHA-256: `c989e7022320afa45eef90ca05afcde1008921e9c095838070ce6acd5e6c0461`
- `Tattler-armv8-0.2.0-0004.spk`
  - generic package arch: `armv8`
  - ELF e_machine: `183`
  - size: `2,877,440` bytes
  - SHA-256: `cc78228b00515e4426dd8f9838e261168f0181edf672336892398282e0c170f1`

The checked-in Package Source copies are byte-for-byte identical to the three CI-built SPKs for this subject.

## Package Source routing

The package catalog maps model/platform-specific Synology architecture identifiers onto the generic package families.

Representative qualified source mappings include:

- `armada38x` → `armv7`
- `armada37xx` → `armv8`
- `rtd1296` → `armv8`
- `geminilake` → `x86_64`

Direct generic requests for `x86_64`, `armv7`, and `armv8` are also supported. Unknown architecture identifiers fail closed and receive no package.

## Verification surfaces

The exact-head workflow passed:

- source tests and vet;
- multi-architecture SPK regression tests;
- lifecycle-hardening regression tests;
- release-document and release-hygiene checks;
- deterministic x86_64 binary double build;
- deterministic ARMv7 binary double build;
- deterministic ARMv8/AArch64 binary double build;
- deterministic SPK double build for all three architectures;
- in-repo SPK verification for all three architectures;
- independent DSM 7.2.2 SPK verification for all three architectures;
- architecture-specific workflow artifact upload for all three SPKs;
- Package Source routing tests;
- byte-for-byte Package Source binding to all three exact CI-built SPKs;
- native Windows runtime smoke;
- Windows desktop tests/build;
- Windows desktop bundle packaging.

## Lifecycle protections retained

All three SPKs retain the `0.2.0-0003` package-lifecycle protections:

- package-PID ownership validation;
- stale PID rejection;
- loopback listener readiness on `127.0.0.1:9147`;
- bounded startup wait and recent-log diagnostics;
- atomic PID-file publication;
- bounded service-log rotation;
- state directory mode `0700`;
- service log mode `0600`;
- package-user-only DSM privilege;
- loopback-only HTTP exposure.

## Evidence ceiling

This receipt qualifies source, deterministic builds, SPK structure, architecture/ELF matching, independent package verification, workflow artifacts, Package Source routing, and Package Source byte provenance for the exact commit above.

It does **not** establish:

- installation of `0.2.0-0004` on the live DS216;
- live Package Center acceptance of the generic `armv7` artifact on the DS216;
- live installation/runtime behavior on an x86_64 DSM host;
- live installation/runtime behavior on an ARMv8 DSM host;
- live upgrade behavior from prior Tattler versions on those architectures.

The live DS216 and any future x86_64/ARMv8 NAS remain separate installation/runtime subjects until explicitly installed and read back.
