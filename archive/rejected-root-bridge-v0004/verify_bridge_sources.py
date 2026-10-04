#!/usr/bin/env python3
from __future__ import annotations

import hashlib
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

FILES = {
    "HELPER_SHA": ROOT / "bridge" / "tattler-pkgctl",
    "SIGNERS_SHA": ROOT / "bridge" / "tattler-release.allowed_signers",
    "SUDOERS_SHA": ROOT / "bridge" / "sudoers.tattler-pkgctl",
}


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main() -> int:
    postinst = (ROOT / "spk" / "scripts" / "postinst").read_text(encoding="utf-8")
    postupgrade = (ROOT / "spk" / "scripts" / "postupgrade").read_text(encoding="utf-8")
    if postinst != postupgrade:
        raise SystemExit("postinst and postupgrade bridge bootstrap scripts differ")

    for variable, path in FILES.items():
        match = re.search(rf'^{variable}="([0-9a-f]{{64}})"$', postinst, re.MULTILINE)
        if not match:
            raise SystemExit(f"{variable} is missing from bridge bootstrap")
        expected = match.group(1)
        actual = sha256(path)
        if expected != actual:
            raise SystemExit(
                f"{variable} mismatch for {path}: bootstrap={expected} source={actual}"
            )

    privilege = (ROOT / "spk" / "conf" / "privilege").read_text(encoding="utf-8")
    if '"run-as": "package"' not in privilege:
        raise SystemExit("package default privilege must remain package")
    for action in ("postinst", "postupgrade", "preuninst"):
        needle = f'"action": "{action}"'
        if needle not in privilege:
            raise SystemExit(f"root lifecycle action missing: {action}")

    print("bridge source pins: PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
