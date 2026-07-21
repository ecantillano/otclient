import os
import subprocess
import tempfile
import textwrap
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
RETRY = ROOT / "scripts" / "retry-cmake-configure.sh"


class RetryCMakeConfigureTests(unittest.TestCase):
    def run_retry(self, command, attempts="3"):
        environment = os.environ.copy()
        environment["THAPPY_CMAKE_CONFIGURE_ATTEMPTS"] = attempts
        environment["THAPPY_CMAKE_CONFIGURE_RETRY_DELAY_SECONDS"] = "0"
        return subprocess.run(
            ["bash", str(RETRY), str(command)],
            check=False,
            capture_output=True,
            text=True,
            env=environment,
        )

    def test_retries_transient_failure_until_success(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            state = root / "attempts"
            command = root / "configure"
            command.write_text(
                textwrap.dedent(
                    """\
                    #!/usr/bin/env bash
                    count=0
                    if [ -f "${STATE_FILE}" ]; then count="$(cat "${STATE_FILE}")"; fi
                    count=$((count + 1))
                    printf '%s' "$count" > "${STATE_FILE}"
                    [ "$count" -ge 3 ]
                    """
                ),
                encoding="utf-8",
            )
            command.chmod(0o755)
            environment = os.environ.copy()
            environment["STATE_FILE"] = str(state)
            environment["THAPPY_CMAKE_CONFIGURE_ATTEMPTS"] = "3"
            environment["THAPPY_CMAKE_CONFIGURE_RETRY_DELAY_SECONDS"] = "0"
            result = subprocess.run(
                ["bash", str(RETRY), str(command)],
                check=False,
                capture_output=True,
                text=True,
                env=environment,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(state.read_text(encoding="utf-8"), "3")

    def test_returns_last_failure_after_attempt_limit(self):
        result = self.run_retry("/usr/bin/false", attempts="2")
        self.assertEqual(result.returncode, 1)
        self.assertIn("failed after 2 attempts", result.stderr)

    def test_rejects_invalid_attempt_count(self):
        result = self.run_retry("/usr/bin/true", attempts="0")
        self.assertEqual(result.returncode, 2)
        self.assertIn("must be a positive integer", result.stderr)


if __name__ == "__main__":
    unittest.main()
