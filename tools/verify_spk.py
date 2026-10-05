#!/usr/bin/env python3
from __future__ import annotations

import argparse
import hashlib
import io
import json
import re
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
    expected = {
        "defaults": {"run-as": "package"},
        "username": "Tattler",
    }
    if privilege != expected:
        raise ValueError(f"privilege must be exactly package-user only: {privilege!r}")

def parse_info(raw: bytes) -> dict[str, str]:
    result: dict[str, str] = {}
    for lineno, line in enumerate(raw.decode("utf-8").splitlines(), 1):
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        if "=" not in line:
            raise ValueError(f"INFO line {lineno} is not key=value")
        key, value = line.split("=", 1)
        key = key.strip()
        value = value.strip()
        if len(value) >= 2 and value[0] == value[-1] == '"':
            value = value[1:-1]
        if key in result:
            raise ValueError(f"duplicate INFO field {key!r}")
        result[key] = value
    return result

def verify_info(info: dict[str, str], package: bytes, payload_bytes: int) -> None:
    required = {
        "package": "Tattler",
        "arch": "armada38x",
        "os_min_ver": "7.2-72806",
        "silent_upgrade": "yes",
        "auto_upgrade_from": "0.1.0-0003",
    }
    for key, expected in required.items():
        actual = info.get(key)
        if actual != expected:
            raise ValueError(f"INFO {key} must be {expected!r}, got {actual!r}")

    version = info.get("version", "")
    if not re.fullmatch(r"\d+\.\d+\.\d+-\d{4}", version):
        raise ValueError(f"INFO version has invalid format: {version!r}")

    checksum = info.get("checksum", "")
    if not re.fullmatch(r"[0-9a-f]{32}", checksum):
        raise ValueError("INFO checksum must be lowercase MD5 hex")
    actual_checksum = hashlib.md5(package, usedforsecurity=False).hexdigest()
    if checksum != actual_checksum:
        raise ValueError(f"INFO checksum mismatch: declared {checksum}, actual {actual_checksum}")

    expected_extractsize = (payload_bytes + 1023) // 1024
    try:
        extractsize = int(info.get("extractsize", ""))
    except ValueError as exc:
        raise ValueError("INFO extractsize must be an integer") from exc
    if extractsize != expected_extractsize:
        raise ValueError(
            f"INFO extractsize mismatch: declared {extractsize}, expected {expected_extractsize}"
        )

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
    info = parse_info(info_raw)

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

    verify_info(info, package, len(main_binary))
    main_machine = verify_arm(main_binary, "tattler")
    return {
        "outer_members": len(outer_members),
        "payload_members": len(inner_members),
        "arm_e_machine": main_machine,
        "package_run_as": "package",
        "privileged_tools": 0,
        "version": info["version"],
    }

def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("spk", type=Path)
    args = parser.parse_args()
    print(verify(args.spk))
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
