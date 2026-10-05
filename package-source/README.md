# Tattler DSM package source

This directory contains two adapters for the same DSM package catalog contract:

- `api/catalog.js`: Vercel-compatible adapter.
- `supabase/index.ts`: Supabase Edge Function adapter.

The Supabase adapter is the active deployment.

## Staged immutable release

The source tree now contains the exact CI-qualified v0007 artifact:

- version: `0.1.0-0007`
- filename: `Tattler-armada38x-0.1.0-0007.spk`
- SHA-256: `a70428e2d9a0f132e5eb3b12d7c7c3608b12601200c023c51354083ea4b8b736`
- MD5: `d22eae80bd6b8226aaa168bdf403910a`
- size: `2,273,280` bytes
- privilege model: package-user only; no root lifecycle actions and no file capabilities

The source-controlled Supabase function points at green immutable release commit `427db77274bc877b2b7d04c4970210bcda237f9e`. Feed-pointer commit `4dda8de50e07673921b696fff11a9fc6774f7961` passed exact-head CI run #32, and deployed Edge Function version 3 now serves that exact v0007 release.

v0006 remains historical publication evidence: DSM discovered it successfully but rejected installation because its `cap_sys_ptrace` helper was classified as root-privileged.

## DSM contract

The endpoint accepts both GET query parameters and POST form parameters.

It returns Tattler only when:

- `arch=armada38x`
- `build>=72806`

The function is intentionally public and read-only. DSM Package Center cannot attach an application JWT to third-party package-source polling, so the deployed Edge Function uses `verify_jwt=false`. The function accepts no mutation method and exposes no database or secret.

Live package-source endpoint:

`https://fawkirqroyniueeqspif.supabase.co/functions/v1/tattler-package-source`

The DS216 already has this source registered as `Tattler`.

## Release process

For a new qualified Tattler version:

1. Build/test the source on exact-head CI.
2. Upload the deterministic CI SPK before the publication-binding check.
3. Download that exact CI artifact.
4. Run Tattler's verifier and the pinned independent `spk-packager` verifier.
5. Run `tools/update_package_source.py` against the CI artifact.
6. Commit the generated release manifest, icons, and exact SPK.
7. Require exact-head CI to reproduce and byte-compare the published SPK.
8. Advance the Edge Function's immutable `SOURCE_COMMIT` only to that green release commit.
9. Deploy a new Edge Function version.
10. Verify GET, POST, returned SPK hash, NAS-side reachability, Package Center discovery, and DSM installation before claiming runtime PASS.
