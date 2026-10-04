#!/usr/bin/env python3
from __future__ import annotations

import argparse
import hashlib
import os
import shutil
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DEFAULT_ALLOWED = ROOT / "bridge" / "tattler-release.allowed_signers"
DEFAULT_KEY = Path.home() / ".ssh" / "tattler_release_ed25519"


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("spk", type=Path)
    parser.add_argument("--key", type=Path, default=DEFAULT_KEY)
    parser.add_argument("--allowed-signers", type=Path, default=DEFAULT_ALLOWED)
    args = parser.parse_args()

    spk = args.spk.resolve()
    key = args.key.expanduser().resolve()
    allowed = args.allowed_signers.resolve()

    if not spk.is_file():
        raise SystemExit(f"SPK not found: {spk}")
    if not key.is_file():
        raise SystemExit(f"release private key not found: {key}")
    if not allowed.is_file():
        raise SystemExit(f"allowed signers file not found: {allowed}")

    ssh_keygen = shutil.which("ssh-keygen")
    if not ssh_keygen:
        raise SystemExit("ssh-keygen is not available")

    signature = Path(str(spk) + ".sig")
    if signature.exists():
        signature.unlink()

    subprocess.run(
        [ssh_keygen, "-Y", "sign", "-f", str(key), "-n", "tattler", str(spk)],
        check=True,
    )
    if not signature.is_file():
        raise SystemExit(f"signature was not created: {signature}")

    with spk.open("rb") as payload:
        result = subprocess.run(
            [
                ssh_keygen,
                "-Y",
                "verify",
                "-f",
                str(allowed),
                "-I",
                "tattler-release",
                "-n",
                "tattler",
                "-s",
                str(signature),
            ],
            stdin=payload,
            check=False,
            text=False,
        )
    if result.returncode != 0:
        raise SystemExit(f"local signature verification failed: rc={result.returncode}")

    digest = hashlib.sha256(spk.read_bytes()).hexdigest()
    print(f"spk={spk}")
    print(f"signature={signature}")
    print(f"sha256={digest}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
