# Tattler DSM package source

This directory contains two adapters for the same DSM package catalog contract:

- `api/catalog.js`: Vercel-compatible adapter.
- `supabase/index.ts`: Supabase Edge Function adapter.

The Vercel connector currently lacks project-creation permission, so the Supabase adapter is the active deployment candidate.

## Current immutable release

The catalog is pinned to Git commit:

`57a82cea61f2a5b9dd1e3c5ddaa90dd0797314af`

That commit contains:

- `release.json`
- the exact CI-qualified v0005 SPK
- 64x64 and 256x256 package icons

The SPK SHA-256 is:

`de78508d584d687de052102b9ac1c01d1344cedb5456466be592f8e04664e50b`

The Supabase function fetches only the pinned release manifest and serves catalog links back to immutable raw GitHub URLs under the same commit.

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
