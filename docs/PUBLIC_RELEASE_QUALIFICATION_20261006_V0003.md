# Tattler DSM SPK 0.2.0-0003 qualification — 2026-10-06

## Exact subject

Source/package subject:

`thebrazenbeard/tattler@5cc51d67efe75bcc6e6a12b4ccde784b23233ae9`

Stacked pull request:

`#4 — improve/spk-v0003-20261006`

GitHub Actions qualification:

- PR run `37541898580`: SUCCESS
- push run `37541894597`: SUCCESS

Both runs executed against the exact subject above.

## Package identity

- package: `Tattler`
- version: `0.2.0-0003`
- architecture: `armada38x`
- DSM minimum: `7.2-72806`
- filename: `Tattler-armada38x-0.2.0-0003.spk`
- size: `3,092,480` bytes
- SHA-256: `3e735b9957fb8852a07024706da37b21354e35a17a58b57765413a8c1bf944bb`

The package-source copy is byte-for-byte identical to the CI-built SPK for this subject.

## Lifecycle hardening qualified

The exact package subject above includes:

- package-PID ownership validation against the resolved installed Tattler binary;
- stale PID rejection before status/stop actions can trust or kill a process;
- loopback listener readiness on `127.0.0.1:9147`;
- bounded startup wait with recent-log diagnostics on failure;
- atomic PID-file publication;
- stale PID cleanup;
- 4 MiB service-log startup rotation with one retained generation;
- install/upgrade state directory mode `0700`;
- install/upgrade service log mode `0600`;
- package-user-only DSM privilege;
- loopback-only HTTP exposure.

The package does not request root lifecycle actions, setuid behavior, Linux file capabilities, firewall changes, or public network exposure.

## Verification surfaces

The exact-head workflow passed:

- source tests and vet;
- SPK lifecycle regression tests;
- deterministic ARMv7 double build;
- deterministic SPK double build;
- strengthened in-repo SPK verification;
- independent DSM 7.2.2 SPK verification using the pinned external verifier;
- package-source tests;
- byte-for-byte package-source binding;
- native Windows runtime smoke;
- Windows desktop tests/build;
- Windows desktop bundle packaging with its sibling agent.

## Evidence ceiling

This receipt qualifies source, build, package structure, deterministic artifact identity, Package Source binding, and CI behavior for the exact commit above.

It does **not** establish:

- installation of `0.2.0-0003` on the live DS216;
- live DSM service startup/stop/upgrade behavior for this revision;
- live Package Center discovery of this exact revision;
- daemon runtime behavior after upgrade.

The live DS216 remains a separate install/runtime subject until an upgrade is explicitly authorized and read back.
