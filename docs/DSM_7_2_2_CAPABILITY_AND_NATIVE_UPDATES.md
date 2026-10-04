# DSM 7.2.2 capability and native-update design

## Why v0004 was rejected

DSM 7.2.2 rejected the first v0004 bridge package because its `conf/privilege` declared root lifecycle actions. Synology's developer documentation states that DSM 7 packages are forced into the lower-privilege package model and that unsigned third-party packages requesting root privilege cannot be installed without Synology signing or a NAS-specific development token issued to collaborative partners.

References:
- https://help.synology.com/developer-guide/getting_started/system_requirement.html
- https://help.synology.com/developer-guide/breaking_changes.html
- https://help.synology.com/developer-guide/privilege/preface.html

That rejected design is preserved under `archive/rejected-root-bridge-v0004/` and is not active.

## Supported file-capability path

Synology documents `conf/privilege.tool[].capabilities` for DSM 7.0-40656 and newer. The package default can remain `run-as: package` while one installed tool receives a Linux file-capability set.

Reference:
- https://help.synology.com/developer-guide/privilege/privilege_config.html

Tattler v0005 therefore introduces:

`bin/tattler-procmap`

with exactly:

```json
{
  "relpath": "bin/tattler-procmap",
  "user": "package",
  "group": "package",
  "capabilities": "cap_sys_ptrace",
  "permission": "0700"
}
```

The main `bin/tattler` daemon receives no file capability. The helper has no network listener, takes only a JSON list of socket inode numbers on stdin, scans `/proc`, and emits inode-to-process metadata on stdout.

The daemon first tries normal package-user process attribution and invokes the helper only for still-unresolved socket inodes. Successful results are cached for the life of the connection.

This is a hypothesis until live DSM readback proves:
- the package installs normally without the root-privilege rejection;
- DSM actually applies `cap_sys_ptrace` to the helper;
- the helper can read the relevant cross-user `/proc/<pid>/fd` links;
- connection API records gain process attribution for Plex/other-user sockets;
- overhead remains acceptable.

## Native upgrade path

Synology documents these INFO fields:

`silent_upgrade="yes"`

Allows upgrade without the package wizard and enables automatic/CMS-driven package upgrades.

`auto_upgrade_from="<version>"`

When combined with `silent_upgrade=yes`, Package Center only auto-upgrades from the specified installed version or newer.

Reference:
- https://help.synology.com/developer-guide/synology_package/INFO_optional_fields.html

Tattler v0005 declares:

```
silent_upgrade="yes"
auto_upgrade_from="0.1.0-0003"
```

This does not itself publish an update. Package Center also needs a third-party package source that advertises a newer compatible Tattler build.

## Tattler package source

`package-source/` implements the DSM package catalog shape used by mature third-party repositories such as SynoCommunity's `spkrepo`.

References:
- https://github.com/SynoCommunity/spkrepo
- https://spkrepo.readthedocs.io/en/latest/api_examples.html
- https://docs.synocommunity.com/developer-guide/publishing/package-server/

The endpoint accepts the DSM architecture/build parameters on GET or POST and returns Tattler only when:
- `arch=armada38x`
- DSM build is `72806` or newer

The response binds Package Center to:
- exact version
- exact SPK filename
- exact MD5
- exact byte size
- package icons
- upgrade/install/start quick flags

`tools/update_package_source.py` derives this metadata from the already-built SPK and copies that exact binary into the package-source release tree. CI re-generates and compares the published source tree so a catalog cannot silently drift from the qualified SPK.

## Current evidence states

Installed live package: `0.1.0-0003`

Current source candidate: `0.1.0-0005`

v0005 source/build/package-source work does not become a runtime PASS until:
1. exact-head CI passes;
2. the exact v0005 package is published by the package source;
3. DSM Package Center discovers the update;
4. DSM installs it without the root-privilege rejection;
5. helper capability is read back on the NAS;
6. cross-user attribution behavior is tested.
