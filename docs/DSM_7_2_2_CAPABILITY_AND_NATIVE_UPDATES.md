# DSM 7.2.2 privilege experiments and native-update design

## v0004 root-lifecycle experiment

DSM 7.2.2 rejected the first v0004 bridge package because its `conf/privilege` declared root lifecycle actions. Synology's developer documentation says DSM 7 packages are forced into the lower-privilege package model and that unsigned third-party packages requesting root privilege cannot be installed without Synology signing or a NAS-specific development token issued to collaborative partners.

References:
- https://help.synology.com/developer-guide/getting_started/system_requirement.html
- https://help.synology.com/developer-guide/breaking_changes.html
- https://help.synology.com/developer-guide/privilege/preface.html

That rejected design is preserved under `archive/rejected-root-bridge-v0004/` and is not active.

## v0006 file-capability experiment

Synology documents `conf/privilege.tool[].capabilities` for DSM 7.0-40656 and newer. v0006 therefore kept the package default at `run-as: package` while assigning only `cap_sys_ptrace` to a tiny sibling helper, `bin/tattler-procmap`.

Reference:
- https://help.synology.com/developer-guide/privilege/privilege_config.html

The exact v0006 SPK passed:
- Tattler's structural verifier;
- `thebrazenbeard/spk-packager@89085efb9e439dfd05f26ad56d857ef71f2d52b1`;
- deterministic CI build and publication binding;
- DSM Package Source discovery.

Live DSM 7.2.2 Package Center nevertheless rejected v0006 before installation with:

> Unable to install Tattler because it runs with root privileges.

Therefore this DS216's installer classifies the capability-bearing package as privileged for unsigned-install purposes. v0006 was never installed and no runtime capability claim is valid.

Live installer behavior supersedes the design assumption.

## v0007 package-user-only attribution

v0007 removes the helper and all file capabilities. Its `conf/privilege` is exactly:

```json
{
  "defaults": {
    "run-as": "package"
  },
  "username": "Tattler"
}
```

The unprivileged replacement uses kernel socket-owner evidence already present in `/proc/net/tcp`, `tcp6`, `udp`, and `udp6`.

Each socket record includes:
- numeric socket UID;
- inode;
- endpoints;
- protocol/state.

Tattler resolves the socket UID through `/etc/passwd` and emits an `owner` account name. Exact PID/name/executable attribution is still populated only when the package user can prove the inode through readable `/proc/<pid>/fd` links.

This intentionally separates:
- **exact process attribution** — PID/name/executable proven from readable FD links;
- **owner attribution** — kernel socket UID mapped to account name.

No heuristic PID is manufactured from UID ownership.

## Native upgrade path

Synology documents:

`silent_upgrade="yes"`

and:

`auto_upgrade_from="<version>"`

Reference:
- https://help.synology.com/developer-guide/synology_package/INFO_optional_fields.html

v0007 keeps:

```
silent_upgrade="yes"
auto_upgrade_from="0.1.0-0003"
```

The DS216 already has the Tattler package source registered:

`https://fawkirqroyniueeqspif.supabase.co/functions/v1/tattler-package-source`

The live feed previously advertised v0006 successfully and Package Center showed `Tattler -> Update Available`; the actual v0006 install was then blocked solely at DSM's privilege gate.

## Package-source contract

`package-source/` implements the DSM package catalog shape used by mature third-party repositories such as SynoCommunity's `spkrepo`.

References:
- https://github.com/SynoCommunity/spkrepo
- https://spkrepo.readthedocs.io/en/latest/api_examples.html
- https://docs.synocommunity.com/developer-guide/publishing/package-server/

The endpoint accepts DSM architecture/build parameters on GET or POST and returns Tattler only when:
- `arch=armada38x`
- DSM build is `72806` or newer

The response binds Package Center to exact version, SPK filename, MD5, byte size, icons, and download URL.

`tools/update_package_source.py` derives publication metadata from the already-built SPK. CI regenerates and byte-compares the published SPK so the catalog cannot silently drift from the qualified artifact.

## Independent package verification

The independent `spk-packager` verifier remains a required CI gate. It previously caught a missing DSM payload checksum in v0005 before installation.

v0007 must pass both Tattler's verifier and the independent strict DSM 7.2.2 verifier before publication.

## Current evidence states

Rejected candidate `0.1.0-0006`:

`BUILD_PASS / PACKAGE_SOURCE_DISCOVERY_PASS / DSM_INSTALL_REJECTED_ROOT_PRIVILEGE_CLASSIFICATION / NEVER_INSTALLED`

Current live subject `0.1.0-0007`:

`PACKAGE_USER_ONLY / UID_OWNER_ATTRIBUTION_IMPLEMENTED / EXACT_HEAD_CI_PASS / PUBLISHED / LIVE_FEED_PASS / PACKAGE_CENTER_UPGRADE_PASS / LIVE_DAEMON_PASS / UID_OWNER_ATTRIBUTION_RUNTIME_PASS`

Publication/runtime evidence:
- release commit `427db77274bc877b2b7d04c4970210bcda237f9e` passed exact-head CI run #30;
- feed-pointer commit `4dda8de50e07673921b696fff11a9fc6774f7961` passed exact-head CI run #32;
- current source head `9bd2c543b66e766bf3098b269b8461fd66f9c508` passed exact-head CI run #34;
- deployed Supabase Edge Function `tattler-package-source` points at the immutable v0007 release;
- live GET and POST return `0.1.0-0007`;
- canonical SPK SHA-256 is `a70428e2d9a0f132e5eb3b12d7c7c3608b12601200c023c51354083ea4b8b736`;
- the DS216 reached the feed and Package Center upgraded from v0003 to v0007 natively;
- direct readback observed v0007 running as user `Tattler`, PID `8972`, with loopback API responsive;
- live API connection records resolved socket UIDs to `http`, `tailscale`, `WorkBridgeRelay`, and `root`;
- exact cross-user `process` attribution remained empty where DSM denied readable FD proof, preserving the intended evidence ceiling;
- immediate post-upgrade overhead was approximately 8 MiB RSS and 1.4-1.5% CPU.

The native DSM update path and UID/account owner-attribution path are therefore runtime-qualified on this DS216. Exact cross-user PID attribution remains deliberately LIMITED rather than inferred.
