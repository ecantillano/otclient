#!/usr/bin/env python3
"""Assert the immutable production endpoint inside the built package."""

import argparse
import json
import sys
from pathlib import Path

from client_release import ReleaseError, audit_archive


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("package", type=Path)
    args = parser.parse_args()
    try:
        result = audit_archive(args.package, require_layout=False, require_production_config=True)
        config_errors = [
            error
            for error in result.errors
            if "production" in error.lower()
            or "canonical" in error.lower()
            or "endpoint" in error.lower()
            or "protocol" in error.lower()
            or "httplogin" in error.lower()
            or "authenticator" in error.lower()
        ]
        if config_errors:
            raise ReleaseError("; ".join(config_errors))
    except (OSError, ReleaseError) as error:
        print("test-production-config: {}".format(error), file=sys.stderr)
        return 1
    print(json.dumps({"ok": True, "package": args.package.name}, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
