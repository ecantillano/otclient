#!/usr/bin/env python3
"""Verify that a package produces a complete clean install."""

import argparse
import json
import sys
from pathlib import Path

from client_release import ReleaseError, verify_clean_install


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("package", type=Path)
    parser.add_argument("--platform", choices=("windows", "linux"), required=True)
    parser.add_argument("--environment", choices=("production", "test", "local"), default="production")
    args = parser.parse_args()
    try:
        result = verify_clean_install(args.package, args.platform, args.environment)
    except (OSError, ReleaseError) as error:
        print("test-clean-install: {}".format(error), file=sys.stderr)
        return 1
    print(json.dumps(result, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
