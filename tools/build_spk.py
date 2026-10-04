#!/usr/bin/env python3
from __future__ import annotations

import argparse
import gzip
import hashlib
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

def validate_arm_elf(data: bytes, label: str) -> None:
    if len(data) < 20 or data[:4] != b"\x7fELF":
        raise ValueError(f"{label} is not ELF")
    endian = "<" if data[5] == 1 else ">"
    machine = struct.unpack(endian + "H", data[18:20])[0]
    if machine != 40:
        raise ValueError(f"{label}: expected ARM ELF e_machine=40, got {machine}")

def build(binary: Path, helper: Path, output: Path) -> None:
    binary_bytes = binary.read_bytes()
    helper_bytes = helper.read_bytes()
    validate_arm_elf(binary_bytes, "tattler")
    validate_arm_elf(helper_bytes, "tattler-procmap")

    payload_raw = io.BytesIO()
    with tarfile.open(fileobj=payload_raw, mode="w:") as inner:
        add_bytes(inner, "bin/tattler", binary_bytes, 0o755)
        add_bytes(inner, "bin/tattler-procmap", helper_bytes, 0o700)

    package_buf = io.BytesIO()
    with gzip.GzipFile(fileobj=package_buf, mode="wb", mtime=0, filename="") as gz:
        gz.write(payload_raw.getvalue())
    package_tgz = package_buf.getvalue()
    package_checksum = hashlib.md5(package_tgz, usedforsecurity=False).hexdigest()
    payload_bytes = len(binary_bytes) + len(helper_bytes)
    extractsize_kb = (payload_bytes + 1023) // 1024

    info_lines = []
    for line in (SPK / "INFO").read_text(encoding="utf-8").splitlines():
        if line.startswith("checksum=") or line.startswith("extractsize="):
            continue
        info_lines.append(line)
    info_lines.append(f'checksum="{package_checksum}"')
    info_lines.append(f'extractsize="{extractsize_kb}"')
    info_bytes = ("\n".join(info_lines) + "\n").encode("utf-8")

    members: list[tuple[str, bytes, int]] = [("INFO", info_bytes, 0o644)]
    for path in sorted(p for p in SPK.rglob("*") if p.is_file()):
        rel = path.relative_to(SPK).as_posix()
        if rel == "INFO":
            continue
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
    parser.add_argument("--helper", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    build(args.binary, args.helper, args.output)
    print(args.output)
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
