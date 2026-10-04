# Tattler

Tattler is a low-overhead Linux/NAS diagnostic agent whose job is to answer a practical question:

> Why is this machine slow right now?

The first deployment target is Synology DSM on DS216-class ARMv7 hardware, but the collector and diagnosis engine are intentionally portable Linux code.

## Design target

Tattler samples kernel and process telemetry, keeps a bounded local history, correlates resource pressure with the processes causing it, and emits human-readable findings instead of requiring the operator to interpret graphs manually.

The initial diagnostic surface is intentionally read-only. Source, a built SPK, an installed package, a running service, and a validated diagnosis are separate states.

## Research baseline

The first implementation is informed by:

- the DSM package/lifecycle and deterministic-build patterns already used by `thebrazenbeard/vera-synology` and `thebrazenbeard/workbridge`;
- SynoCommunity `spksrc` conventions for DSM package metadata and service lifecycle;
- Beszel and Prometheus node_exporter patterns for lightweight Linux host metrics;
- Scrutiny's approach to optional SMART collection without assuming a single drive interface.

No third-party source is vendored by default. See `docs/RESEARCH.md` on the build branch for exact architectural decisions and provenance notes.

## Status

Bootstrap only on `main`. Active implementation work belongs on an isolated build branch until reviewed.
