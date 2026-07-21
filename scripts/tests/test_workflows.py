import re
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
WORKFLOWS = ROOT / ".github" / "workflows"


class WorkflowContractTests(unittest.TestCase):
    def read(self, name):
        return (WORKFLOWS / name).read_text(encoding="utf-8")

    def test_all_actions_are_official_and_commit_pinned(self):
        for name in ("client-ci.yml", "client-release.yml", "update-manifest.yml"):
            text = self.read(name)
            uses = re.findall(r"uses:\s+([^@\s]+)@([^\s#]+)", text)
            self.assertTrue(uses, name)
            for action, revision in uses:
                self.assertTrue(action.startswith("actions/"), (name, action))
                self.assertRegex(revision, r"^[0-9a-f]{40}$", (name, action, revision))

    def test_ci_builds_named_binaries_and_runs_cpp_tests(self):
        text = self.read("client-ci.yml")
        self.assertIn("build/windows-release/bin/Thappy.exe", text)
        self.assertIn("build/linux-release/bin/thappy", text)
        self.assertIn("cmake --preset linux-debug", text)
        self.assertIn("ctest --preset linux-debug", text)
        self.assertIn("Verify vcpkg manifest baseline", text)
        self.assertIn("--budget 104857600", text)
        self.assertGreaterEqual(text.count('export VCPKG_ROOT="${VCPKG_INSTALLATION_ROOT}"'), 2)
        self.assertIn("releases/download/client-v${VERSION}", text)
        self.assertNotIn("contents: write", text)

    def test_release_contract_is_non_publishing_and_immutable(self):
        text = self.read("client-release.yml")
        self.assertIn('tags:\n      - "client-v*"', text)
        self.assertIn("client-vX.Y.Z or client-vX.Y.Z-rc.N", text)
        self.assertIn("Release ${tag} already exists; refusing to overwrite.", text)
        self.assertIn("Release ${RELEASE_TAG} already exists; refusing to overwrite.", text)
        self.assertIn("manifest-${CHANNEL}.json", text)
        self.assertIn("Verify vcpkg manifest baseline", text)
        self.assertIn("--budget 104857600", text)
        self.assertIn("--draft", text)
        self.assertIn("--prerelease", text)
        self.assertEqual(text.count("contents: write"), 1)
        self.assertNotIn("releases/download/v${", text)

    def test_manifest_update_only_touches_protected_channel_manifest(self):
        text = self.read("update-manifest.yml")
        self.assertIn("manifest-${CHANNEL}.json", text)
        self.assertIn("isDraft", text)
        self.assertIn("isPrerelease", text)
        self.assertIn("already exists; refusing to overwrite", text)
        self.assertEqual(text.count("contents: write"), 1)


if __name__ == "__main__":
    unittest.main()
