#!/usr/bin/env python3
from __future__ import annotations

import argparse
import io
import struct
import tarfile
from pathlib import Path

REQUIRED_OUTER = {
    "INFO", "PACKAGE_ICON.PNG", "PACKAGE_ICON_256.PNG",
    "conf/privilege", "conf/resource", "package.tgz",
    "scripts/start-stop-status", "scripts/preinst", "scripts/postinst",
    "scripts/preuninst", "scripts/postuninst", "scripts/preupgrade", "scripts/postupgrade",
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

    verify_png(icon_64, 64, 64, "PACKAGE_ICON.PNG")
    verify_png(icon_256, 256, 256, "PACKAGE_ICON_256.PNG")

    with tarfile.open(fileobj=io.BytesIO(package), mode="r:gz") as inner:
        inner_members = safe_members(inner)
        names = {m.name for m in inner_members}
        if names != {"bin/tattler"}:
            raise ValueError(f"unexpected payload members: {sorted(names)}")
        binary = inner.extractfile("bin/tattler").read()

    if len(binary) < 20 or binary[:4] != b"\x7fELF":
        raise ValueError("payload binary is not ELF")
    endian = "<" if binary[5] == 1 else ">"
    machine = struct.unpack(endian + "H", binary[18:20])[0]
    if machine != 40:
        raise ValueError(f"payload binary is not ARM: e_machine={machine}")
    return {
        "outer_members": len(outer_members),
        "payload_members": len(inner_members),
        "arm_e_machine": machine,
    }

def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("spk", type=Path)
    args = parser.parse_args()
    print(verify(args.spk))
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
