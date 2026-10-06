#!/usr/bin/env python3
from __future__ import annotations

import argparse
import hashlib
import json
import shutil
import tarfile
from pathlib import Path

SUPPORTED_ARCHES = {"x86_64", "armv7", "armv8"}


def parse_info(spk: Path) -> dict[str, str]:
    with tarfile.open(spk, "r:") as tf:
        raw = tf.extractfile("INFO").read().decode("utf-8")
    out: dict[str, str] = {}
    for line in raw.splitlines():
        if "=" not in line:
            continue
        key, value = line.split("=", 1)
        value = value.strip()
        if len(value) >= 2 and value[0] == value[-1] == '"':
            value = value[1:-1]
        out[key.strip()] = value
    return out


def digest(path: Path, name: str) -> str:
    h = hashlib.new(name)
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def stage_many(spks: list[Path], output: Path, icons_dir: Path) -> dict:
    if not spks:
        raise SystemExit("at least one SPK is required")

    parsed: list[tuple[Path, dict[str, str]]] = []
    versions: set[str] = set()
    arches: set[str] = set()

    for spk in spks:
        info = parse_info(spk)
        if info.get("package") != "Tattler":
            raise SystemExit(f"{spk}: SPK package id is not Tattler")
        arch = info.get("arch", "")
        if arch not in SUPPORTED_ARCHES:
            raise SystemExit(f"{spk}: unsupported SPK arch {arch!r}")
        if arch in arches:
            raise SystemExit(f"duplicate SPK arch: {arch}")
        version = info.get("version", "")
        if not version:
            raise SystemExit(f"{spk}: SPK version missing")
        arches.add(arch)
        versions.add(version)
        parsed.append((spk, info))

    if arches != SUPPORTED_ARCHES:
        missing = sorted(SUPPORTED_ARCHES - arches)
        extra = sorted(arches - SUPPORTED_ARCHES)
        raise SystemExit(f"SPK architecture set incomplete: missing={missing}, extra={extra}")
    if len(versions) != 1:
        raise SystemExit(f"SPK versions do not match: {sorted(versions)}")

    version = versions.pop()
    release_dir = output / "public" / "releases"
    icon_dir = output / "public" / "icons"
    release_dir.mkdir(parents=True, exist_ok=True)
    icon_dir.mkdir(parents=True, exist_ok=True)

    for old in release_dir.glob("Tattler-*.spk"):
        old.unlink()

    releases = []
    for spk, info in sorted(parsed, key=lambda item: item[1]["arch"]):
        arch = info["arch"]
        filename = f"Tattler-{arch}-{version}.spk"
        target = release_dir / filename
        shutil.copy2(spk, target)
        releases.append(
            {
                "arch": arch,
                "filename": filename,
                "size": target.stat().st_size,
                "md5": digest(target, "md5"),
                "sha256": digest(target, "sha256"),
            }
        )

    shutil.copy2(icons_dir / "PACKAGE_ICON.PNG", icon_dir / "PACKAGE_ICON.PNG")
    shutil.copy2(icons_dir / "PACKAGE_ICON_256.PNG", icon_dir / "PACKAGE_ICON_256.PNG")

    manifest = {
        "package": "Tattler",
        "version": version,
        "min_build": 72806,
        "releases": releases,
    }
    (output / "release.json").write_text(
        json.dumps(manifest, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
        newline="\n",
    )
    return manifest


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--spk", type=Path, action="append", required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--icons-dir", type=Path, required=True)
    args = parser.parse_args()
    manifest = stage_many(
        [path.resolve() for path in args.spk],
        args.output.resolve(),
        args.icons_dir.resolve(),
    )
    print(json.dumps(manifest, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
