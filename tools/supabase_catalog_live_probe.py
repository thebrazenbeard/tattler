#!/usr/bin/env python3
"""Opt-in, read-only smoke probe for Tattler's deployed DSM catalog.

The offline tests import probe() with a fake HTTP transport. Network access
occurs only when this file is invoked with --live. No credentials are used.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from typing import Callable
from urllib.parse import urlencode, urlsplit
from urllib.request import HTTPRedirectHandler, Request, build_opener

DEFAULT_ENDPOINT = (
    "https://fawkirqroyniueeqspif.supabase.co/functions/v1/tattler-package-source"
)
DEPLOYED_SOURCE_COMMIT = "427db77274bc877b2b7d04c4970210bcda237f9e"
EXPECTED_PROJECT_REF = "fawkirqroyniueeqspif"
MAX_RESPONSE_BYTES = 65536
HttpGet = Callable[[str], tuple[int, object, dict[str, str]]]


class ProbeError(RuntimeError):
    """The read-only service contract was not satisfied."""


class RejectRedirects(HTTPRedirectHandler):
    """A different effective responder must never qualify the pinned provider."""

    def redirect_request(self, request, fp, code, msg, headers, newurl):
        raise ProbeError(f"catalog HTTP {code} redirect refused")


def live_get(url: str) -> tuple[int, object, dict[str, str]]:
    request = Request(
        url,
        method="GET",
        headers={"Accept": "application/json", "User-Agent": "Tattler-ReadOnly-Probe/1"},
    )
    # urllib.urlopen follows redirects by default. Use a rejecting handler
    # and verify the effective URL even if a transport/proxy misbehaves.
    with build_opener(RejectRedirects()).open(request, timeout=12) as response:
        if response.geturl() != url:
            raise ProbeError("catalog effective response URL differs from requested URL")
        raw = response.read(MAX_RESPONSE_BYTES + 1)
        if len(raw) > MAX_RESPONSE_BYTES:
            raise ProbeError("catalog response exceeds the 64-KiB probe limit")
        try:
            body = json.loads(raw)
        except (UnicodeDecodeError, json.JSONDecodeError) as exc:
            raise ProbeError("catalog response is not valid JSON") from exc
        return response.status, body, dict(response.headers.items())


def packages_at(
    endpoint: str, arch: str, build: int, http_get: HttpGet
) -> tuple[list[object], dict[str, str]]:
    separator = "&" if "?" in endpoint else "?"
    url = f"{endpoint}{separator}{urlencode({'arch': arch, 'build': build})}"
    status, body, headers = http_get(url)
    if status != 200:
        raise ProbeError(f"catalog returned HTTP {status}, expected 200")
    if not isinstance(body, dict) or not isinstance(body.get("packages"), list):
        raise ProbeError("catalog response lacks a packages array")
    return body["packages"], headers


def probe(
    endpoint: str,
    source_commit: str,
    version: str,
    arch: str,
    min_build: int,
    http_get: HttpGet,
) -> dict[str, object]:
    url = urlsplit(endpoint)
    if url.scheme != "https" or not url.netloc or url.username or url.password:
        raise ProbeError("endpoint must be HTTPS without URL credentials")
    if url.query or url.fragment:
        raise ProbeError("endpoint must not contain a query or fragment")
    if endpoint != DEFAULT_ENDPOINT:
        raise ProbeError("live provider attribution requires the exact approved Supabase endpoint")
    if not re.fullmatch(r"[0-9a-f]{40}", source_commit):
        raise ProbeError("source commit must be an exact 40-character SHA")
    if not version or min_build < 1:
        raise ProbeError("expected release parameters are invalid")

    found, headers = packages_at(endpoint, arch, min_build, http_get)
    if headers.get("sb-project-ref") != EXPECTED_PROJECT_REF:
        raise ProbeError("response does not identify the approved Supabase project")
    if len(found) != 1 or not isinstance(found[0], dict):
        raise ProbeError("supported architecture must return exactly one package")
    package = found[0]
    if package.get("package") != "Tattler" or package.get("version") != version:
        raise ProbeError("deployed package identity/version differs from expectation")
    prefix = (
        "https://raw.githubusercontent.com/thebrazenbeard/tattler/"
        f"{source_commit}/package-source/public/releases/"
    )
    link = package.get("link")
    if not isinstance(link, str) or not link.startswith(prefix):
        raise ProbeError("package artifact is not pinned to the expected immutable commit")
    filename = link[len(prefix) :]
    if not re.fullmatch(r"[A-Za-z0-9._-]+\.spk", filename):
        raise ProbeError("artifact filename is not a simple .spk basename")
    if not isinstance(package.get("size"), int) or package["size"] <= 0:
        raise ProbeError("artifact size is invalid")
    if not isinstance(package.get("md5"), str) or not re.fullmatch(
        r"[0-9a-f]{32}", package["md5"]
    ):
        raise ProbeError("artifact MD5 metadata is malformed")

    unknown, _ = packages_at(endpoint, "unrecognized-tattler-arch", min_build, http_get)
    if unknown:
        raise ProbeError("unknown architecture was offered a package")
    old, _ = packages_at(endpoint, arch, min_build - 1, http_get)
    if old:
        raise ProbeError("below-minimum DSM build was offered a package")

    return {
        "result": "LIVE_PROVIDER_HTTP_PASS",
        "scope": "three read-only catalog GETs; not installation or artifact digest proof",
        "expected_commit": source_commit,
        "version": version,
        "architecture": arch,
        "artifact": filename,
        "edge_region": headers.get("x-sb-edge-region", "unknown"),
        "provider_request_id": headers.get("sb-request-id", "unavailable"),
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--live", action="store_true", help="allow three real, read-only HTTP GETs")
    parser.add_argument("--endpoint", default=DEFAULT_ENDPOINT)
    parser.add_argument("--expected-commit", default=DEPLOYED_SOURCE_COMMIT)
    parser.add_argument("--expected-version", default="0.1.0-0007")
    parser.add_argument("--arch", default="armada38x")
    parser.add_argument("--min-build", type=int, default=72806)
    args = parser.parse_args()
    if not args.live:
        parser.error("live network probe is opt-in: supply --live; otherwise run the offline unit tests")
    try:
        result = probe(
            args.endpoint,
            args.expected_commit,
            args.expected_version,
            args.arch,
            args.min_build,
            live_get,
        )
    except (OSError, ProbeError) as exc:
        print(json.dumps({"result": "FAIL", "reason": str(exc)}), file=sys.stderr)
        return 1
    print(json.dumps(result, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
