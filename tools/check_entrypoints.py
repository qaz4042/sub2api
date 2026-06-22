#!/usr/bin/env python3
"""Validate repository entrypoints that are easy to drift.

This check intentionally focuses on project-owned command/documentation entrypoints:
- Makefile recipes that invoke local scripts must point to existing files.
- The root Makefile must not advertise datamanagement build/test targets because
  this repository does not contain datamanagementd source.
- datamanagementd docs and install help must not reference the old in-repo
  ./datamanagement/datamanagementd path or a local go build flow.
"""

from __future__ import annotations

import re
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]

REQUIRED_FILES = (
    Path("tools/secret_scan.py"),
    Path("deploy/install-datamanagementd.sh"),
    Path("deploy/sub2api-datamanagementd.service"),
    Path("deploy/DATAMANAGEMENTD_CN.md"),
)

TEXT_ENTRYPOINTS = (
    Path("Makefile"),
    Path("deploy/README.md"),
    Path("deploy/DATAMANAGEMENTD_CN.md"),
    Path("deploy/install-datamanagementd.sh"),
)

STALE_PATTERNS = (
    (
        re.compile(r"\./datamanagement/datamanagementd"),
        "references removed in-repo datamanagementd binary path",
    ),
    (
        re.compile(r"\bgo\s+build\b.*datamanagement", re.IGNORECASE),
        "documents or invokes an in-repo datamanagementd build",
    ),
    (
        re.compile(r"\b(?:build|test)[-_]?datamanagement(?:d)?\b", re.IGNORECASE),
        "advertises datamanagement build/test target without source in this repo",
    ),
)

MAKEFILE_LOCAL_SCRIPT_PATTERNS = (
    re.compile(r"(?:^|\s)(?:@)?(?:\.\/)([\w./-]+\.sh)\b"),
    re.compile(r"(?:^|\s)(?:@)?python3\s+([\w./-]+\.py)\b"),
)


def rel(path: Path) -> str:
    return path.relative_to(ROOT).as_posix()


def fail(errors: list[str], message: str) -> None:
    errors.append(message)


def check_required_files(errors: list[str]) -> None:
    for path in REQUIRED_FILES:
        full_path = ROOT / path
        if not full_path.is_file():
            fail(errors, f"missing required entrypoint file: {path.as_posix()}")


def check_makefile_local_scripts(errors: list[str]) -> None:
    makefile = ROOT / "Makefile"
    if not makefile.is_file():
        fail(errors, "missing root Makefile")
        return

    for line_number, line in enumerate(makefile.read_text(encoding="utf-8").splitlines(), start=1):
        for pattern in MAKEFILE_LOCAL_SCRIPT_PATTERNS:
            for match in pattern.finditer(line):
                candidate = ROOT / match.group(1)
                if not candidate.is_file():
                    fail(
                        errors,
                        f"Makefile:{line_number} references missing local entrypoint: {match.group(1)}",
                    )


def check_stale_text(errors: list[str]) -> None:
    for path in TEXT_ENTRYPOINTS:
        full_path = ROOT / path
        if not full_path.is_file():
            continue
        for line_number, line in enumerate(full_path.read_text(encoding="utf-8").splitlines(), start=1):
            for pattern, reason in STALE_PATTERNS:
                if pattern.search(line):
                    fail(errors, f"{path.as_posix()}:{line_number} {reason}")


def check_shell_syntax(errors: list[str]) -> None:
    for path in (Path("deploy/install-datamanagementd.sh"),):
        full_path = ROOT / path
        if not full_path.is_file():
            continue
        result = subprocess.run(["bash", "-n", str(full_path)], cwd=ROOT)
        if result.returncode != 0:
            fail(errors, f"shell syntax check failed: {path.as_posix()}")


def main() -> int:
    errors: list[str] = []
    check_required_files(errors)
    check_makefile_local_scripts(errors)
    check_stale_text(errors)
    check_shell_syntax(errors)

    if errors:
        for error in errors:
            print(error, file=sys.stderr)
        print(f"entrypoint check failed: {len(errors)} issue(s)", file=sys.stderr)
        return 1

    print("entrypoint check passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
