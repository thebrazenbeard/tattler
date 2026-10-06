#!/usr/bin/env python3
from __future__ import annotations

import argparse
import ipaddress
import re
from dataclasses import dataclass, field
from pathlib import Path


PRIVATE_KEY_RE = re.compile(
    r"-----BEGIN (?:RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----",
    re.IGNORECASE,
)
TOKEN_RES = (
    ("openai-token", re.compile(r"\bsk-[A-Za-z0-9_-]{32,}\b")),
    ("github-token", re.compile(r"\bgh[pousr]_[A-Za-z0-9]{30,}\b")),
)
SECRET_ASSIGNMENT_RE = re.compile(
    r"""(?ix)
    \b(api[_-]?key|secret|token|password)\b
    \s*[:=]\s*
    ["']?([^\s"'#;,]{12,})
    """
)
PLACEHOLDER_MARKERS = (
    "example", "placeholder", "changeme", "your-", "your_", "<", "${", "$env:",
)
WINDOWS_USER_RE = re.compile(r"\b[A-Za-z]:\\Users\\[^\\\s]+", re.IGNORECASE)
VERA_PATH_RE = re.compile(r"\bD:\\VERA(?:\\[^\s]*)?", re.IGNORECASE)
IPV4_RE = re.compile(r"(?<![0-9.])(?:\d{1,3}\.){3}\d{1,3}(?![0-9.])")
RFC1918 = (
    ipaddress.ip_network("10.0.0.0/8"),
    ipaddress.ip_network("172.16.0.0/12"),
    ipaddress.ip_network("192.168.0.0/16"),
)

SKIP_DIRS = {".git", "dist", "node_modules", "__pycache__"}
SKIP_FILES = {"tools/release_hygiene.py", "tools/release_hygiene_test.py"}
SKIP_SUFFIXES = {
    ".png", ".jpg", ".jpeg", ".gif", ".ico", ".spk", ".exe", ".dll",
    ".zip", ".gz", ".tgz", ".tar", ".pdf",
}


@dataclass
class Finding:
    path: str
    line: int
    kind: str
    excerpt: str


@dataclass
class ScanResult:
    blockers: list[Finding] = field(default_factory=list)
    review: list[Finding] = field(default_factory=list)


def _placeholder(value: str) -> bool:
    lowered = value.lower()
    return any(marker in lowered for marker in PLACEHOLDER_MARKERS)


def _text_files(root: Path):
    for path in root.rglob("*"):
        if not path.is_file():
            continue
        rel = path.relative_to(root)
        rel_text = str(rel).replace("\\", "/")
        if rel_text in SKIP_FILES:
            continue
        if any(part in SKIP_DIRS for part in rel.parts):
            continue
        if path.suffix.lower() in SKIP_SUFFIXES:
            continue
        try:
            raw = path.read_bytes()
        except OSError:
            continue
        if b"\x00" in raw:
            continue
        try:
            text = raw.decode("utf-8")
        except UnicodeDecodeError:
            continue
        yield rel, text


def _finding(rel: Path, line_no: int, kind: str, line: str) -> Finding:
    clean = line.strip()
    if len(clean) > 180:
        clean = clean[:177] + "..."
    return Finding(str(rel).replace("\\", "/"), line_no, kind, clean)


def scan_tree(root: Path) -> ScanResult:
    result = ScanResult()
    for rel, text in _text_files(root):
        for line_no, line in enumerate(text.splitlines(), start=1):
            if PRIVATE_KEY_RE.search(line):
                result.blockers.append(_finding(rel, line_no, "private-key-header", line))

            for kind, regex in TOKEN_RES:
                if regex.search(line) and not _placeholder(line):
                    result.blockers.append(_finding(rel, line_no, kind, line))

            for match in SECRET_ASSIGNMENT_RE.finditer(line):
                value = match.group(2)
                if not _placeholder(value):
                    result.blockers.append(_finding(rel, line_no, "secret-assignment", line))
                    break

            if WINDOWS_USER_RE.search(line):
                result.review.append(_finding(rel, line_no, "windows-user-path", line))
            if VERA_PATH_RE.search(line):
                result.review.append(_finding(rel, line_no, "vera-path", line))

            for match in IPV4_RE.finditer(line):
                candidate = match.group(0)
                try:
                    address = ipaddress.ip_address(candidate)
                except ValueError:
                    continue
                if any(address in network for network in RFC1918):
                    result.review.append(_finding(rel, line_no, "private-ip", line))
    return result


def _print_findings(title: str, findings: list[Finding]) -> None:
    print(f"{title}: {len(findings)}")
    for item in findings:
        print(f"  {item.path}:{item.line} [{item.kind}] {item.excerpt}")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", type=Path, required=True)
    args = parser.parse_args()

    result = scan_tree(args.check.resolve())
    _print_findings("BLOCKERS", result.blockers)
    _print_findings("REVIEW", result.review)
    return 1 if result.blockers else 0


if __name__ == "__main__":
    raise SystemExit(main())
