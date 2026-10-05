# Rejected v0004 root-bridge design

This directory preserves the first privileged bridge design as historical evidence only.

DSM 7.2.2 Package Center rejected the corresponding SPK before installation because the package declared root lifecycle actions. Synology's DSM 7 developer documentation states that unsigned third-party packages requesting root privilege cannot be installed unless they are Synology-signed or a NAS-specific development token has been issued to a collaborative partner.

The active Tattler design supersedes this approach:
- package default remains `run-as: package`;
- no root lifecycle action is requested;
- cross-process socket attribution is isolated in a tiny capability-bearing helper using DSM's documented `conf/privilege.tool[].capabilities` mechanism;
- updates use DSM's native package-source plus `silent_upgrade` / `auto_upgrade_from` path.

Nothing in this archived directory is packaged, installed, or invoked by the active build.
