#!/usr/bin/env bash
set -euo pipefail

attempts="${THAPPY_CMAKE_CONFIGURE_ATTEMPTS:-3}"
delay_seconds="${THAPPY_CMAKE_CONFIGURE_RETRY_DELAY_SECONDS:-15}"

if ! [[ "$attempts" =~ ^[1-9][0-9]*$ ]]; then
  echo "THAPPY_CMAKE_CONFIGURE_ATTEMPTS must be a positive integer" >&2
  exit 2
fi
if ! [[ "$delay_seconds" =~ ^[0-9]+$ ]]; then
  echo "THAPPY_CMAKE_CONFIGURE_RETRY_DELAY_SECONDS must be a non-negative integer" >&2
  exit 2
fi
if [ "$#" -eq 0 ]; then
  echo "usage: retry-cmake-configure.sh COMMAND [ARG ...]" >&2
  exit 2
fi

attempt=1
while [ "$attempt" -le "$attempts" ]; do
  echo "CMake configure attempt ${attempt}/${attempts}"
  if "$@"; then
    exit 0
  else
    status=$?
  fi

  if [ "$attempt" -eq "$attempts" ]; then
    echo "CMake configure failed after ${attempts} attempts" >&2
    exit "$status"
  fi

  echo "CMake configure failed with status ${status}; retrying in ${delay_seconds}s" >&2
  if [ "$delay_seconds" -gt 0 ]; then
    sleep "$delay_seconds"
  fi
  attempt=$((attempt + 1))
done
