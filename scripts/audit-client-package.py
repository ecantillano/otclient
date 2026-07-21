#!/usr/bin/env python3
"""Audit the actual contents of a built Thappy package."""

import argparse
import json
import sys
from pathlib import Path

from client_release import ReleaseError, audit_archive, write_json


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("package", type=Path)
    parser.add_argument("--environment", choices=("production", "test", "local"), default="production")
    parser.add_argument("--json-output", type=Path)
    parser.add_argument("--component", action="store_true", help="do not require full-package layout")
    args = parser.parse_args()
    try:
        result = audit_archive(
            args.package,
            require_layout=not args.component,
            require_production_config=args.environment == "production" and not args.component,
            allow_private_endpoints=args.environment != "production",
        )
    except (OSError, ReleaseError) as error:
        print("audit-client-package: {}".format(error), file=sys.stderr)
        return 1
    payload = result.as_dict()
    if args.json_output:
        write_json(args.json_output, payload)
    print(json.dumps(payload, indent=2, sort_keys=True))
    return 0 if result.ok else 1


if __name__ == "__main__":
    sys.exit(main())
