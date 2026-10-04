# Tattler DSM package source

This directory contains two adapters for the same DSM package catalog contract:

- `api/catalog.js`: Vercel-compatible adapter.
- `supabase/index.ts`: Supabase Edge Function adapter.

The Vercel connector lacks project-creation permission, so the Supabase adapter is the active deployment.

## Current immutable release

The catalog is pinned to Git commit:

`7d3c1561b8d6f64133641121c7cc451771bf4d33`

**Compatibility note:** this currently published v0006 release is discoverable by DSM but was rejected before installation because DSM classified its `cap_sys_ptrace` helper as root-privileged. It is retained as publication evidence only while the package-user-only v0007 replacement is being qualified.

That commit contains:

- `release.json`
- the exact CI-qualified v0006 SPK
- 64x64 and 256x256 package icons

The SPK SHA-256 is:

`fdb3bf8b4d358cca397b4688d3e25ce01d1ef8df43f3a7d4d5a965eb4c1ee22c`

The Supabase function fetches only the pinned release manifest and serves catalog links back to immutable raw GitHub URLs under the same commit. Deployment readback on 2026-10-04 confirmed both DSM-style GET and POST catalog requests return Tattler 0.1.0-0006, and the DS216 itself can reach the feed and download the complete 3,164,160-byte SPK body.

Live package-source endpoint: `https://fawkirqroyniueeqspif.supabase.co/functions/v1/tattler-package-source`

## DSM contract

The endpoint accepts both GET query parameters and POST form parameters because mature Synology package repositories support both forms.

It returns Tattler only when:

- `arch=armada38x`
- `build>=72806`

The function is intentionally public and read-only. A DSM Package Source cannot provide a JWT when Package Center polls it, so the deployed Edge Function must use `verify_jwt=false`. The function accepts no mutation method and exposes no database or secret.

## Release process

For a new qualified Tattler version:

1. Build and structurally verify the SPK.
2. Run `tools/update_package_source.py` against that exact SPK.
3. Commit the generated release manifest, icons, and SPK.
4. Require CI to reproduce and byte-compare the published SPK.
5. Advance the Edge Function's immutable `SOURCE_COMMIT` to the commit containing the qualified release payload.
6. Deploy a new function version.
7. Verify Package Center discovers the update before claiming publication PASS.
