# Tattler privileged package bridge

## Purpose

DSM 7.2.2 requires root for package-management operations such as `synopkg install`. Tattler itself must remain unprivileged. The bridge exists only to let the already-authorized operator account `psims85` perform a tiny set of Tattler package lifecycle operations noninteractively over the existing SSH key.

The bridge is not a general root shell and does not grant passwordless `sudo` for arbitrary commands.

## Authority boundary

The installed root-owned executable is:

`/usr/local/sbin/tattler-pkgctl`

The only sudo rules are:

- `tattler-pkgctl status`
- `tattler-pkgctl start`
- `tattler-pkgctl stop`
- `tattler-pkgctl upgrade <sha256>`

The helper itself accepts no arbitrary package path. Upgrade input is fixed to:

- `/volume1/homes/psims85/.tattler-upload/Tattler.spk`
- `/volume1/homes/psims85/.tattler-upload/Tattler.spk.sig`

## Upgrade trust chain

For an upgrade to run as root, all of the following must pass:

1. Caller is exactly `psims85` invoking the helper through sudo.
2. Staging directory is the fixed Tattler upload directory and is owned by `psims85`.
3. SPK and signature are regular, non-symlink files owned by `psims85` with exactly one hard link.
4. The helper copies both files into a private root-owned temporary directory before verification, closing the writable-source race.
5. The copied SPK SHA-256 must equal the 64-hex digest supplied on the command line.
6. DSM's own `ssh-keygen -Y verify` must validate an Ed25519 signature for principal `tattler-release`, namespace `tattler`, against the root-owned allowed-signers file.
7. The SPK outer `INFO` must declare `package="Tattler"`, `arch="armada38x"`, and a nonempty version.
8. The candidate version must differ from the currently installed version.
9. Only then may the helper invoke `/usr/syno/bin/synopkg install` on the root-private copy.
10. The installed `/var/packages/Tattler/INFO` version is read back and must match the candidate.

Operations are serialized with a root-owned flock and logged through syslog plus `/var/log/tattler-pkgctl.log`.

## Signing key

The release private key is deliberately outside Git:

`C:\Users\patri\.ssh\tattler_release_ed25519`

The NAS receives only the public signer policy. Current public-key fingerprint:

`SHA256:ooKT0ZZErpgzZjQe9mFB26Lkl1bQpGeOkukBCbF2Gm8`

A built SPK is signed with:

```bash
python tools/sign_spk.py dist/Tattler-armada38x-<version>.spk
```

CI can build and structurally verify an SPK but cannot manufacture a bridge-approved release signature because CI does not have the private key.

## Bootstrap

DSM lifecycle defaults remain `run-as: package`. Only these lifecycle actions run as root:

- `postinst`
- `postupgrade`
- `preuninst`

`postinst` and `postupgrade` verify pinned SHA-256 values for the bridge helper, signer policy, and sudoers policy before copying them to root-owned locations. The sudoers snippet is installed mode 0440 and immediately checked with `sudo -l -U psims85`; failure rolls the policy back.

`preuninst` removes the helper, signer policy, and sudoers rule but preserves the audit log.

## Evidence ceiling

Source/package verification does not prove the bridge is installed or effective. The bridge becomes runtime-qualified only after an exact v0004 package is installed on the DS216 and direct readback proves:

- root-owned helper and trust store permissions;
- exact sudoers contents;
- `sudo -n tattler-pkgctl status` works as `psims85`;
- an unsigned or bad-signature package is rejected;
- a correctly signed newer Tattler package can be installed without browser/password involvement.
