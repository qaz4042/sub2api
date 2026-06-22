#!/usr/bin/env python3
"""Scan tracked text files for high-confidence credential signatures."""

from __future__ import annotations

import re
import subprocess
import sys
from pathlib import Path


PATTERNS = {
    "private key": re.compile(rb"-----BEGIN (?:[A-Z0-9 ]+ )?PRIVATE KEY-----"),
    "AWS access key": re.compile(rb"\bAKIA[0-9A-Z]{16}\b"),
    "GitHub token": re.compile(rb"\bgh[pousr]_[A-Za-z0-9]{30,}\b"),
    "Slack token": re.compile(rb"\bxox[baprs]-[A-Za-z0-9-]{20,}\b"),
    "OpenAI-style key": re.compile(rb"\bsk-[A-Za-z0-9_-]{24,}\b"),
}

MAX_FILE_SIZE = 2 * 1024 * 1024


def is_known_fixture(line: bytes, label: str) -> bool:
    """Allow only visibly synthetic, inline test values."""
    if label == "private key":
        return any(
            marker in line
            for marker in (b"...", b"\\nabc\\n", b"\\nMIIE\\n", b"\\ndata\\n", b"\\\\nMIIE\\\\n")
        )
    if label == "OpenAI-style key":
        return any(
            marker in line
            for marker in (b"sk-update-last-used-", b"sk-getbykey-auth-dispatch-unit")
        )
    return False


def tracked_files() -> list[Path]:
    result = subprocess.run(
        ["git", "ls-files", "-z"],
        check=True,
        stdout=subprocess.PIPE,
    )
    return [Path(raw.decode("utf-8")) for raw in result.stdout.split(b"\0") if raw]


def main() -> int:
    findings: list[tuple[Path, int, str]] = []
    for path in tracked_files():
        try:
            if not path.is_file() or path.stat().st_size > MAX_FILE_SIZE:
                continue
            data = path.read_bytes()
        except OSError:
            continue
        if b"\0" in data[:8192]:
            continue
        for line_number, line in enumerate(data.splitlines(), start=1):
            for label, pattern in PATTERNS.items():
                if pattern.search(line) and not is_known_fixture(line, label):
                    findings.append((path, line_number, label))

    if findings:
        for path, line_number, label in findings:
            print(f"{path}:{line_number}: possible {label}", file=sys.stderr)
        print(f"secret scan failed: {len(findings)} finding(s)", file=sys.stderr)
        return 1

    print(f"secret scan passed: {len(tracked_files())} tracked files checked")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
