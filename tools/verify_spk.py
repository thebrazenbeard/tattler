#!/usr/bin/env python3
from __future__ import annotations

import argparse
import io
import json
import struct
import tarfile
from pathlib import Path

REQUIRED_OUTER = {
    "INFO", "PACKAGE_ICON.PNG", "PACKAGE_ICON_256.PNG",
    "conf/privilege", "conf/resource", "package.tgz",
    "scripts/start-stop-status", "scripts/preinst", "scripts/postinst",
    "scripts/preuninst", "scripts/postuninst", "scripts/preupgrade", "scripts/postupgrade",
}

EXPECTED_INNER_MODES = {
    "bin/tattler": 0o755,
    "share/tattler-pkgctl/allowed_signers": 0o644,
    "share/tattler-pkgctl/sudoers.tattler-pkgctl": 0o644,
    "share/tattler-pkgctl/tattler-pkgctl": 0o755,
}

EXPECTED_ROOT_ACTIONS = {"postinst", "postupgrade", "preuninst"}


def safe_members(tf: tarfile.TarFile) -> list[tarfile.TarInfo]:
    members = tf.getmembers()
    names = [m.name for m in members]
    if len(names) != len(set(names)):
        raise ValueError("duplicate archive members")
    if names != sorted(names):
        raise ValueError("archive members are not sorted")
    for member in members:
        path = Path(member.name)
        if path.is_absolute() or ".." in path.parts or not member.isfile():
            raise ValueError(f"unsafe member: {member.name}")
        if member.uid != 0 or member.gid != 0 or member.mtime != 0:
            raise ValueError(f"non-deterministic metadata: {member.name}")
    return members


def verify_png(data: bytes, width: int, height: int, name: str) -> None:
    signature = b"\x89PNG\r\n\x1a\n"
    if len(data) < 24 or data[:8] != signature or data[12:16] != b"IHDR":
        raise ValueError(f"{name} is not a valid PNG")
    actual = struct.unpack(">II", data[16:24])
    if actual != (width, height):
        raise ValueError(f"{name} must be {width}x{height}, got {actual[0]}x{actual[1]}")


def verify_privilege(raw: bytes) -> None:
    privilege = json.loads(raw)
    defaults = privilege.get("defaults", {})
    if defaults.get("run-as") != "package":
        raise ValueError("Tattler daemon/default lifecycle must remain package-user")
    root_actions = {
        entry.get("action")
        for entry in privilege.get("ctrl-script", [])
        if entry.get("run-as") == "root"
    }
    if root_actions != EXPECTED_ROOT_ACTIONS:
        raise ValueError(
            f"root lifecycle actions must be exactly {sorted(EXPECTED_ROOT_ACTIONS)}, got {sorted(root_actions)}"
        )
    for entry in privilege.get("ctrl-script", []):
        if entry.get("run-as") not in {"package", "root"}:
            raise ValueError(f"unexpected lifecycle run-as: {entry!r}")


def verify(path: Path) -> dict[str, int]:
    with tarfile.open(path, "r:") as outer:
        outer_members = safe_members(outer)
        names = {m.name for m in outer_members}
        missing = REQUIRED_OUTER - names
        if missing:
            raise ValueError(f"missing outer members: {sorted(missing)}")
        package = outer.extractfile("package.tgz").read()
        icon_64 = outer.extractfile("PACKAGE_ICON.PNG").read()
        icon_256 = outer.extractfile("PACKAGE_ICON_256.PNG").read()
        privilege_raw = outer.extractfile("conf/privilege").read()

    verify_png(icon_64, 64, 64, "PACKAGE_ICON.PNG")
    verify_png(icon_256, 256, 256, "PACKAGE_ICON_256.PNG")
    verify_privilege(privilege_raw)

    with tarfile.open(fileobj=io.BytesIO(package), mode="r:gz") as inner:
        inner_members = safe_members(inner)
        names = {m.name for m in inner_members}
        expected = set(EXPECTED_INNER_MODES)
        if names != expected:
            raise ValueError(
                f"unexpected payload members: expected {sorted(expected)}, got {sorted(names)}"
            )
        by_name = {m.name: m for m in inner_members}
        for name, mode in EXPECTED_INNER_MODES.items():
            actual_mode = by_name[name].mode & 0o777
            if actual_mode != mode:
                raise ValueError(
                    f"{name} mode must be {oct(mode)}, got {oct(actual_mode)}"
                )
        binary = inner.extractfile("bin/tattler").read()
        allowed_signers = inner.extractfile("share/tattler-pkgctl/allowed_signers").read().decode("ascii")
        sudoers = inner.extractfile("share/tattler-pkgctl/sudoers.tattler-pkgctl").read().decode("ascii")

    if len(binary) < 20 or binary[:4] != b"\x7fELF":
        raise ValueError("payload binary is not ELF")
    endian = "<" if binary[5] == 1 else ">"
    machine = struct.unpack(endian + "H", binary[18:20])[0]
    if machine != 40:
        raise ValueError(f"payload binary is not ARM: e_machine={machine}")

    if not allowed_signers.startswith("tattler-release ssh-ed25519 "):
        raise ValueError("release signer policy is not the expected Ed25519 principal")

    sudo_lines = [line for line in sudoers.splitlines() if line.strip()]
    if len(sudo_lines) != 4:
        raise ValueError("sudoers policy must contain exactly four non-empty rules")
    required_commands = {"status", "start", "stop", "upgrade *"}
    seen_commands = set()
    prefix = "psims85 ALL=(root) NOPASSWD: /usr/local/sbin/tattler-pkgctl "
    for line in sudo_lines:
        if not line.startswith(prefix):
            raise ValueError(f"unexpected sudoers rule: {line}")
        seen_commands.add(line[len(prefix):])
    if seen_commands != required_commands:
        raise ValueError(f"sudoers commands mismatch: {sorted(seen_commands)}")

    return {
        "outer_members": len(outer_members),
        "payload_members": len(inner_members),
        "arm_e_machine": machine,
        "root_actions": len(EXPECTED_ROOT_ACTIONS),
        "sudo_rules": len(sudo_lines),
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("spk", type=Path)
    args = parser.parse_args()
    print(verify(args.spk))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
