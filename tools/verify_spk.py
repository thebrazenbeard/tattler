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
    "bin/tattler-procmap": 0o700,
}

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

def verify_arm(binary: bytes, label: str) -> int:
    if len(binary) < 20 or binary[:4] != b"\x7fELF":
        raise ValueError(f"{label} is not ELF")
    endian = "<" if binary[5] == 1 else ">"
    machine = struct.unpack(endian + "H", binary[18:20])[0]
    if machine != 40:
        raise ValueError(f"{label} is not ARM: e_machine={machine}")
    return machine

def verify_privilege(raw: bytes) -> None:
    privilege = json.loads(raw)
    defaults = privilege.get("defaults", {})
    if defaults != {"run-as": "package"}:
        raise ValueError(f"package defaults must be exactly run-as package: {defaults!r}")
    if privilege.get("username") != "Tattler":
        raise ValueError("package username must be Tattler")
    if privilege.get("ctrl-script"):
        raise ValueError("root/control-script privilege overrides are forbidden")

    tools = privilege.get("tool", [])
    if len(tools) != 1:
        raise ValueError("exactly one capability-bearing helper is required")
    tool = tools[0]
    expected = {
        "relpath": "bin/tattler-procmap",
        "user": "package",
        "group": "package",
        "capabilities": "cap_sys_ptrace",
        "permission": "0700",
    }
    if tool != expected:
        raise ValueError(f"unexpected helper privilege declaration: {tool!r}")

def verify_info(raw: bytes) -> None:
    text = raw.decode("utf-8")
    required = {
        'package="Tattler"',
        'version="0.1.0-0005"',
        'arch="armada38x"',
        'os_min_ver="7.2-72806"',
        'silent_upgrade="yes"',
        'auto_upgrade_from="0.1.0-0003"',
    }
    lines = set(text.splitlines())
    missing = sorted(required - lines)
    if missing:
        raise ValueError(f"required INFO fields missing: {missing}")

def verify(path: Path) -> dict[str, int | str]:
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
        info_raw = outer.extractfile("INFO").read()

    verify_png(icon_64, 64, 64, "PACKAGE_ICON.PNG")
    verify_png(icon_256, 256, 256, "PACKAGE_ICON_256.PNG")
    verify_privilege(privilege_raw)
    verify_info(info_raw)

    with tarfile.open(fileobj=io.BytesIO(package), mode="r:gz") as inner:
        inner_members = safe_members(inner)
        names = {m.name for m in inner_members}
        expected = set(EXPECTED_INNER_MODES)
        if names != expected:
            raise ValueError(f"unexpected payload members: expected {sorted(expected)}, got {sorted(names)}")
        by_name = {m.name: m for m in inner_members}
        for name, mode in EXPECTED_INNER_MODES.items():
            actual_mode = by_name[name].mode & 0o777
            if actual_mode != mode:
                raise ValueError(f"{name} mode must be {oct(mode)}, got {oct(actual_mode)}")
        main_binary = inner.extractfile("bin/tattler").read()
        helper_binary = inner.extractfile("bin/tattler-procmap").read()

    main_machine = verify_arm(main_binary, "tattler")
    helper_machine = verify_arm(helper_binary, "tattler-procmap")
    return {
        "outer_members": len(outer_members),
        "payload_members": len(inner_members),
        "arm_e_machine": main_machine,
        "helper_e_machine": helper_machine,
        "package_run_as": "package",
        "helper_capability": "cap_sys_ptrace",
    }

def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("spk", type=Path)
    args = parser.parse_args()
    print(verify(args.spk))
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
