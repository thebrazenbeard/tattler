#!/usr/bin/env python3
from __future__ import annotations

import argparse
import hashlib
import json
import shutil
import tarfile
from pathlib import Path


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


def stage(spk: Path, output: Path, icons_dir: Path) -> dict:
    info = parse_info(spk)
    if info.get("package") != "Tattler":
        raise SystemExit("SPK package id is not Tattler")
    if info.get("arch") != "armada38x":
        raise SystemExit("SPK arch is not armada38x")
    version = info.get("version")
    if not version:
        raise SystemExit("SPK version missing")

    release_dir = output / "public" / "releases"
    icon_dir = output / "public" / "icons"
    release_dir.mkdir(parents=True, exist_ok=True)
    icon_dir.mkdir(parents=True, exist_ok=True)

    filename = f"Tattler-armada38x-{version}.spk"
    target = release_dir / filename

    for old in release_dir.glob("Tattler-armada38x-*.spk"):
        if old.name != filename:
            old.unlink()
    shutil.copy2(spk, target)
    shutil.copy2(icons_dir / "PACKAGE_ICON.PNG", icon_dir / "PACKAGE_ICON.PNG")
    shutil.copy2(icons_dir / "PACKAGE_ICON_256.PNG", icon_dir / "PACKAGE_ICON_256.PNG")

    manifest = {
        "package": "Tattler",
        "version": version,
        "arch": "armada38x",
        "min_build": 72806,
        "filename": filename,
        "size": target.stat().st_size,
        "md5": digest(target, "md5"),
        "sha256": digest(target, "sha256"),
    }
    (output / "release.json").write_text(
        json.dumps(manifest, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
        newline="\n",
    )
    return manifest


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--spk", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--icons-dir", type=Path, required=True)
    args = parser.parse_args()
    manifest = stage(args.spk.resolve(), args.output.resolve(), args.icons_dir.resolve())
    print(json.dumps(manifest, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
