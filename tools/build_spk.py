#!/usr/bin/env python3
from __future__ import annotations

import argparse
import gzip
import io
import struct
import tarfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SPK = ROOT / "spk"

def tarinfo(name: str, size: int, mode: int) -> tarfile.TarInfo:
    item = tarfile.TarInfo(name)
    item.size = size
    item.mode = mode
    item.uid = item.gid = 0
    item.uname = item.gname = ""
    item.mtime = 0
    return item

def add_bytes(tf: tarfile.TarFile, name: str, data: bytes, mode: int) -> None:
    tf.addfile(tarinfo(name, len(data), mode), io.BytesIO(data))

def validate_arm_elf(data: bytes) -> None:
    if len(data) < 20 or data[:4] != b"\x7fELF":
        raise ValueError("binary is not ELF")
    endian = "<" if data[5] == 1 else ">"
    machine = struct.unpack(endian + "H", data[18:20])[0]
    if machine != 40:
        raise ValueError(f"expected ARM ELF e_machine=40, got {machine}")

def build(binary: Path, output: Path) -> None:
    binary_bytes = binary.read_bytes()
    validate_arm_elf(binary_bytes)

    payload_raw = io.BytesIO()
    with tarfile.open(fileobj=payload_raw, mode="w:") as inner:
        add_bytes(inner, "bin/tattler", binary_bytes, 0o755)
        bridge_files = [
            ("share/tattler-pkgctl/allowed_signers", ROOT / "bridge" / "tattler-release.allowed_signers", 0o644),
            ("share/tattler-pkgctl/sudoers.tattler-pkgctl", ROOT / "bridge" / "sudoers.tattler-pkgctl", 0o644),
            ("share/tattler-pkgctl/tattler-pkgctl", ROOT / "bridge" / "tattler-pkgctl", 0o755),
        ]
        for name, source, mode in bridge_files:
            add_bytes(inner, name, source.read_bytes(), mode)

    package_buf = io.BytesIO()
    with gzip.GzipFile(fileobj=package_buf, mode="wb", mtime=0, filename="") as gz:
        gz.write(payload_raw.getvalue())
    package_tgz = package_buf.getvalue()

    members: list[tuple[str, bytes, int]] = []
    for path in sorted(p for p in SPK.rglob("*") if p.is_file()):
        rel = path.relative_to(SPK).as_posix()
        mode = 0o755 if rel.startswith("scripts/") else 0o644
        members.append((rel, path.read_bytes(), mode))
    members.append(("package.tgz", package_tgz, 0o644))
    members.sort(key=lambda item: item[0])

    output.parent.mkdir(parents=True, exist_ok=True)
    with tarfile.open(output, mode="w:") as outer:
        for name, data, mode in members:
            add_bytes(outer, name, data, mode)

def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    build(args.binary, args.output)
    print(args.output)
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
