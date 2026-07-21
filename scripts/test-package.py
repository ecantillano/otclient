#!/usr/bin/env python3
"""Gate a built package on contents, measured budget, and baseline regression."""

import argparse
import json
import sys
from pathlib import Path

from client_release import ReleaseError, audit_archive, enforce_size_gate


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("package", type=Path)
    parser.add_argument("--environment", choices=("production", "test", "local"), default="production")
    parser.add_argument("--budget", type=int)
    parser.add_argument("--baseline-report", type=Path)
    args = parser.parse_args()
    try:
        result = audit_archive(
            args.package,
            require_layout=True,
            require_production_config=args.environment == "production",
            allow_private_endpoints=args.environment != "production",
        )
        if not result.ok:
            raise ReleaseError("; ".join(result.errors))
        baseline = None
        if args.baseline_report:
            baseline_payload = json.loads(args.baseline_report.read_text(encoding="utf-8"))
            baseline = baseline_payload.get("compressed_size")
            if not isinstance(baseline, int) or baseline <= 0:
                raise ReleaseError("baseline report has invalid compressed_size")
        gate = enforce_size_gate(args.package.stat().st_size, args.budget, baseline)
    except (OSError, ValueError, ReleaseError) as error:
        print("test-package: {}".format(error), file=sys.stderr)
        return 1
    print(json.dumps({"ok": True, "audit": result.as_dict(), "size_gate": gate}, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
