# DSM 7.2.2 package contract notes

Research target: Synology DSM 7.2.2 package lifecycle as documented in the current DSM 7 third-party package developer guide and cross-checked against SynoCommunity `spksrc`.

## Relevant contract

- DSM 7 requires `conf/privilege` and forces third-party packages to run as the package identity rather than root by default.
- `precheckstartstop="yes"` causes Package Center to call `start-stop-status prestart` before `start`, and `prestop` before `stop`.
- A nonzero `prestart` result blocks package start.
- `status` has defined exit codes: 0 running, 1 dead with PID file, 2 dead with lock file, 3 not running, 4 unknown, 150 broken/reinstall.
- Package lifecycle scripts receive `SYNOPKG_PKGDEST`, `SYNOPKG_PKGVAR`, DSM version fields, and `SYNOPKG_TEMP_LOGFILE`.
- `conf/resource` is optional unless a package requests DSM-managed system resources.

## V0.1.0-0001 defect

Tattler declared `precheckstartstop="yes"` but its lifecycle script handled only `start`, `stop`, and `status`. On DSM 7.2.2, `prestart` therefore fell into the default branch and exited nonzero before the daemon could launch.

Its stopped `status` path also returned shell status 1 rather than DSM status 3 when no PID file existed.

## V0.1.0-0002 correction

- Implements successful `prestart` and `prestop` handlers.
- `prestart` validates that the package target/var environment exists and that the Tattler binary is executable.
- Implements DSM status semantics: 0 running, 1 stale/dead PID, 3 cleanly stopped.
- On daemon start failure, removes the stale PID and copies the tail of the service log to `SYNOPKG_TEMP_LOGFILE` when DSM provides it.
- Adds a `log` action that returns the service-log path.
- Keeps the service unprivileged; no root privilege or DSM resource worker is added.
