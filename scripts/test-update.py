#!/usr/bin/env python3
"""Simulate component update, deletion, integrity, and preservation policy."""

import argparse
import json
import sys
from pathlib import Path

from client_release import ReleaseError, simulate_update


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", type=Path, required=True)
    parser.add_argument("--package-dir", type=Path, required=True)
    parser.add_argument("--platform", choices=("windows", "linux"), required=True)
    args = parser.parse_args()
    try:
        result = simulate_update(args.manifest, args.package_dir, args.platform)
    except (OSError, ReleaseError) as error:
        print("test-update: {}".format(error), file=sys.stderr)
        return 1
    print(json.dumps(result, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
