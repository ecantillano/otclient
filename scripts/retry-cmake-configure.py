#!/usr/bin/env python3
"""Retry a CMake configure command while preserving its dependency cache."""

import os
import subprocess
import sys
import time


def parse_integer(name: str, default: int, minimum: int) -> int:
    raw = os.environ.get(name, str(default))
    try:
        value = int(raw)
    except ValueError as error:
        raise ValueError("{} must be an integer".format(name)) from error
    if value < minimum:
        qualifier = "positive" if minimum == 1 else "non-negative"
        raise ValueError("{} must be a {} integer".format(name, qualifier))
    return value


def main() -> int:
    try:
        attempts = parse_integer("THAPPY_CMAKE_CONFIGURE_ATTEMPTS", 3, 1)
        delay_seconds = parse_integer("THAPPY_CMAKE_CONFIGURE_RETRY_DELAY_SECONDS", 15, 0)
    except ValueError as error:
        print(error, file=sys.stderr)
        return 2

    command = sys.argv[1:]
    if not command:
        print("usage: retry-cmake-configure.py COMMAND [ARG ...]", file=sys.stderr)
        return 2

    last_status = 1
    for attempt in range(1, attempts + 1):
        print("CMake configure attempt {}/{}".format(attempt, attempts), flush=True)
        completed = subprocess.run(command, check=False)
        last_status = completed.returncode
        if last_status == 0:
            return 0
        if attempt == attempts:
            break
        print(
            "CMake configure failed with status {}; retrying in {}s".format(
                last_status, delay_seconds
            ),
            file=sys.stderr,
            flush=True,
        )
        if delay_seconds:
            time.sleep(delay_seconds)

    print("CMake configure failed after {} attempts".format(attempts), file=sys.stderr)
    return last_status


if __name__ == "__main__":
    sys.exit(main())
