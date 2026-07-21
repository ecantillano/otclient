import json
import os
import subprocess
import sys
import tempfile
import unittest
import zipfile
from pathlib import Path

SCRIPTS = Path(__file__).resolve().parents[1]
if str(SCRIPTS) not in sys.path:
    sys.path.insert(0, str(SCRIPTS))

from client_release import (  # noqa: E402
    ArchiveEntry,
    ReleaseError,
    audit_archive,
    audit_entries,
    build_packages,
    enforce_size_gate,
    is_safe_bot_purge_migration,
    is_safe_launcher_purge_config,
    merge_manifests,
    read_archive,
    safe_archive_path,
    simulate_update,
    validate_manifest,
    verify_clean_install,
)


PRODUCTION_CONFIG = """
return {
  production = {
    loginUrl = 'https://login.thappy.cl/login.php',
    loginPort = 443,
    protocolVersion = 1525,
    httpLogin = false,
    useAuthenticator = false,
    updateManifestUrl = 'https://github.com/ecantillano/otclient/releases/latest/download/manifest-stable.json',
    servers = { ['https://login.thappy.cl/login.php'] = {
      port = 443, protocol = 1525, httpLogin = false, useAuthenticator = false
    } }
  }
}
"""

SAFE_MIGRATION = """
local migration = {}
local removedBotKeys = {
  'game_bot',
  'vBot',
  'cavebot',
  'targetbot',
  'default_configs'
}
function migration.run(settings)
  for _, key in ipairs(removedBotKeys) do
    settings.remove(key)
  end
end
return migration
"""


def write(path: Path, data, binary=False):
    path.parent.mkdir(parents=True, exist_ok=True)
    if binary:
        path.write_bytes(data)
    else:
        path.write_text(data, encoding="utf-8")


def source_fixture(root: Path):
    write(root / "init.lua", "local e = dofile('/config/environments.lua')\nreturn e\n")
    write(root / "otclientrc.lua", "return true\n")
    write(root / "config.ini", "[graphics]\n")
    write(root / "cacert.pem", "TEST CERTIFICATE\n")
    write(root / "LICENSE", "MIT License\n")
    write(root / "THIRD_PARTY_NOTICES.md", "Third party notices\n")
    write(root / "licenses" / "OTClient-MIT.txt", "MIT License\n")
    write(root / "config" / "environments.lua", PRODUCTION_CONFIG)
    write(root / "config" / "settings_migration.lua", SAFE_MIGRATION)
    write(root / "config" / "retro_modules.lua", "return {}\n")
    write(root / "config" / "build_config.lua", "return { environment = 'production' }\n")
    for module in ("client", "corelib", "gamelib", "modulelib", "startup", "game_interface"):
        write(root / "modules" / module / (module + ".lua"), "return true\n")
    write(root / "modules" / "game_wheel" / "wheel.lua", "return 'modern'\n")
    write(root / "mods" / "game_bot" / "bot.lua", "return 'bot'\n")
    write(root / "data" / "styles" / "base.otui", "Panel {}\n")
    write(root / "data" / "images" / "icon.png", b"PNG", binary=True)
    write(root / "data" / "images" / "game" / "wheel" / "modern.png", b"MODERN", binary=True)
    write(root / "data" / "images" / "options" / "bot.png", b"BOT", binary=True)
    write(root / "data" / "images" / "game" / "slots" / "source.psd", b"PSD", binary=True)
    write(root / "data" / "locales" / "es.lua", "return { label = 'botón' }\n")
    write(root / "data" / "things" / "README.md", "not an authorized asset\n")
    client = root / "build" / "otclient.exe"
    launcher = root / "launcher-build" / "ThappyLauncher.exe"
    write(client, b"MZrelease-client", binary=True)
    write(launcher, b"MZrelease-launcher", binary=True)
    return client, launcher


class PackagingTests(unittest.TestCase):
    def test_windows_package_components_manifest_and_clean_install(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / "source"
            client, launcher = source_fixture(root)
            output = Path(temporary) / "dist"
            result = build_packages(
                root,
                client,
                launcher,
                output,
                "windows",
                "1.2.3",
                "stable",
                "https://github.com/ecantillano/otclient/releases/download/client-v1.2.3",
                1700000000,
                environment="production",
                asset_version="1525",
                build_commit="a" * 40,
            )
            self.assertEqual(set(result["components"]), {"bootstrap", "core", "modules", "data"})
            full = Path(result["full_archive"])
            archive_entries = read_archive(full)
            names = {entry.name for entry in archive_entries}
            modes = {entry.name: entry.mode for entry in archive_entries}
            entries = {entry.name: entry.data for entry in archive_entries}
            self.assertIn("Thappy.exe", names)
            self.assertIn("ThappyLauncher.exe", names)
            self.assertIn("launcher-config.json", names)
            self.assertTrue(modes["Thappy.exe"] & 0o111)
            self.assertTrue(modes["ThappyLauncher.exe"] & 0o111)
            self.assertFalse(any(name.startswith("mods/") for name in names))
            self.assertFalse(any(name.startswith("modules/game_wheel/") for name in names))
            self.assertNotIn("data/things/README.md", names)
            self.assertNotIn("data/images/game/wheel/modern.png", names)
            self.assertNotIn("data/images/options/bot.png", names)
            self.assertNotIn("data/images/game/slots/source.psd", names)
            marker = json.loads(next(entry.data for entry in read_archive(full) if entry.name == "artifact-environment.json"))
            self.assertEqual(marker["environment"], "production")
            manifest = json.loads(Path(result["manifest"]).read_text(encoding="utf-8"))
            validate_manifest(manifest)
            self.assertNotIn("bootstrap-windows", {component["name"] for component in manifest["components"]})
            launcher_config = json.loads(entries["launcher-config.json"])
            self.assertEqual(launcher_config["default_channel"], "stable")
            self.assertEqual(
                launcher_config["channels"]["stable"]["manifest_url"],
                "https://github.com/ecantillano/otclient/releases/latest/download/manifest-stable.json",
            )
            self.assertEqual(
                manifest["release_notes_url"],
                "https://github.com/ecantillano/otclient/releases/tag/client-v1.2.3",
            )
            for field in (
                "protocol_version",
                "asset_version",
                "release_notes_url",
                "mandatory",
                "minimum_launcher_version",
                "environment",
                "login_url",
                "login_port",
                "http_login",
                "use_authenticator",
            ):
                self.assertIn(field, manifest)
            self.assertTrue(audit_archive(full).ok)
            self.assertTrue(verify_clean_install(full, "windows")["ok"])
            self.assertTrue(simulate_update(Path(result["manifest"]), output, "windows")["ok"])
            report = json.loads(Path(result["size_report"]).read_text(encoding="utf-8"))
            self.assertEqual(report["size_gate"]["budget"], None)
            self.assertEqual(report["size_gate"]["measured_budget"], int(full.stat().st_size * 1.10))
            self.assertEqual(set(report["components"]), {"bootstrap", "core", "modules", "data"})
            self.assertTrue(all(value["size_gate"]["budget"] is None for value in report["components"].values()))
            self.assertEqual(len(Path(result["checksums"]).read_text(encoding="utf-8").splitlines()), 8)
            sbom = json.loads(Path(result["sbom"]).read_text(encoding="utf-8"))
            file_ids = [item["SPDXID"] for item in sbom["files"]]
            self.assertEqual(len(file_ids), len(set(file_ids)))
            self.assertRegex(
                sbom["packages"][0]["packageVerificationCode"]["packageVerificationCodeValue"],
                r"^[0-9a-f]{40}$",
            )
            repeated = build_packages(
                root,
                client,
                launcher,
                Path(temporary) / "dist-repeat",
                "windows",
                "1.2.3",
                "stable",
                "https://github.com/ecantillano/otclient/releases/download/client-v1.2.3",
                1700000000,
                environment="production",
                asset_version="1525",
                build_commit="a" * 40,
            )
            self.assertEqual(full.read_bytes(), Path(repeated["full_archive"]).read_bytes())

    def test_production_rejects_all_endpoint_overrides(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / "source"
            client, launcher = source_fixture(root)
            with self.assertRaisesRegex(ReleaseError, "immutable"):
                build_packages(
                    root,
                    client,
                    launcher,
                    Path(temporary) / "dist",
                    "windows",
                    "1.0.0",
                    "stable",
                    "https://example.com/releases/v1.0.0",
                    1700000000,
                    environment="production",
                    login_url="https://evil.example/login.php",
                )

    def test_test_profile_requires_explicit_complete_endpoint(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / "source"
            client, launcher = source_fixture(root)
            with self.assertRaisesRegex(ReleaseError, "explicit values"):
                build_packages(
                    root,
                    client,
                    launcher,
                    Path(temporary) / "dist",
                    "windows",
                    "1.0.0-test.1",
                    "test",
                    "https://example.com/releases/v1.0.0-test.1",
                    1700000000,
                    environment="test",
                    login_url="https://test.example/login.php",
                    login_port=443,
                )

    def test_test_profile_is_staged_without_mutating_source(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / "source"
            client, launcher = source_fixture(root)
            original = (root / "config" / "environments.lua").read_bytes()
            result = build_packages(
                root,
                client,
                launcher,
                Path(temporary) / "dist",
                "windows",
                "1.0.0-test.1",
                "test",
                "https://example.com/releases/v1.0.0-test.1",
                1700000000,
                environment="test",
                login_url="https://test.example/login.php",
                login_port=443,
                protocol_version=1525,
                http_login=False,
                use_authenticator=False,
                website_url="https://test.example",
                support_url="https://test.example/support",
                update_manifest_url="https://test.example/manifest.json",
            )
            self.assertEqual((root / "config" / "environments.lua").read_bytes(), original)
            entries = {entry.name: entry.data for entry in read_archive(Path(result["full_archive"]))}
            self.assertIn(b"test.example/login.php", entries["config/environments.lua"])
            self.assertNotIn(b"server =", entries["config/build_config.lua"])
            self.assertIn(b"environment = \"test\"", entries["config/build_config.lua"])
            self.assertTrue(verify_clean_install(Path(result["full_archive"]), "windows", "test")["ok"])


class AuditTests(unittest.TestCase):
    def entry(self, name, data=b"safe"):
        return ArchiveEntry(name, len(data), len(data), 0o644, data)

    def errors(self, entries, production=False):
        return audit_entries(entries, "fixture.zip", 1, False, production).errors

    def test_forbidden_artifacts_and_modules(self):
        cases = {
            "symbols/client.pdb": "debug/build",
            "data/images/game/slots/slots.psd": "development/build",
            "modules/game_wheel/wheel.lua": "modern UI",
            "mods/client_mods/profile.lua": "mods are not mounted",
            "src/client.cpp": "development/build",
            "build/cache.bin": "development/build",
            "data/images/game/cyclopedia/on.png": "modern/BOT visual",
            "data/images/topbuttons/bot.png": "modern/BOT visual",
        }
        for path, expected in cases.items():
            with self.subTest(path=path):
                self.assertTrue(any(expected in error for error in self.errors([self.entry(path)])))

    def test_real_bot_identifiers_fail_but_locale_button_text_passes(self):
        self.assertFalse(self.errors([self.entry("data/locales/es.lua", "botón botón".encode("utf-8"))]))
        self.assertFalse(self.errors([self.entry("data/locales/pt.lua", "botão".encode("utf-8"))]))
        errors = self.errors([self.entry("modules/game_interface/ui.lua", b"modules.game_bot.showBotButton()")])
        self.assertTrue(any("BOT reference" in error for error in errors))

    def test_purge_only_migration_exception_is_semantic(self):
        self.assertTrue(is_safe_bot_purge_migration("config/settings_migration.lua", SAFE_MIGRATION))
        self.assertFalse(
            is_safe_bot_purge_migration(
                "config/settings_migration.lua",
                SAFE_MIGRATION.replace("settings.remove(key)", "g_game.attack(targetbot)"),
            )
        )
        self.assertFalse(is_safe_bot_purge_migration("modules/other.lua", SAFE_MIGRATION))
        self.assertFalse(self.errors([self.entry("config/settings_migration.lua", SAFE_MIGRATION.encode())]))

    def test_launcher_config_allows_bot_name_only_in_delete_policy(self):
        safe = {"delete_allowlist": ["mods/game_bot/"], "channels": {"stable": {}}}
        text = json.dumps(safe)
        self.assertTrue(is_safe_launcher_purge_config("launcher-config.json", text))
        self.assertFalse(self.errors([self.entry("launcher-config.json", text.encode())]))
        unsafe = json.dumps({"delete_allowlist": [], "command": "modules.game_bot.start()"})
        self.assertFalse(is_safe_launcher_purge_config("launcher-config.json", unsafe))

    def test_duplicate_libraries_debug_and_private_endpoint(self):
        duplicates = [self.entry("bin/foo.dll", b"one"), self.entry("plugins/FOO.DLL", b"two")]
        self.assertTrue(any("duplicate runtime library" in error for error in self.errors(duplicates)))
        self.assertTrue(any("debug" in error.lower() for error in self.errors([self.entry("thappy", b"\x7fELF.debug_info") ])))
        self.assertTrue(any("private" in error.lower() for error in self.errors([self.entry("config.lua", b"localhost") ])))

    def test_wrong_production_config(self):
        wrong = PRODUCTION_CONFIG.replace("protocol = 1525", "protocol = 1098")
        errors = self.errors([self.entry("config/environments.lua", wrong.encode())], production=True)
        self.assertTrue(any("wrong" in error or "non-1525" in error for error in errors))

    def test_production_requires_stable_channel_manifest(self):
        wrong = PRODUCTION_CONFIG.replace("manifest-stable.json", "manifest.json")
        errors = self.errors([self.entry("config/environments.lua", wrong.encode())], production=True)
        self.assertTrue(any("stable manifest URL" in error for error in errors))

    def test_safe_archive_path(self):
        for path in ("../evil", "/absolute", "C:/evil", "a/../../evil"):
            with self.subTest(path=path), self.assertRaises(ReleaseError):
                safe_archive_path(path)


class ManifestAndBudgetTests(unittest.TestCase):
    def manifest(self, name="core-windows"):
        return {
            "schema_version": 1,
            "channel": "stable",
            "version": "1.2.3",
            "protocol_version": 1525,
            "asset_version": "1525",
            "release_notes_url": "https://github.com/ecantillano/otclient/releases/tag/client-v1.2.3",
            "mandatory": False,
            "minimum_launcher_version": "0.1.0",
            "environment": "production",
            "login_url": "https://login.thappy.cl/login.php",
            "login_port": 443,
            "http_login": False,
            "use_authenticator": False,
            "published_at": "2026-01-01T00:00:00Z",
            "components": [
                {
                    "name": name,
                    "version": "1.2.3",
                    "platforms": ["windows"],
                    "url": "https://example.com/{}.zip".format(name),
                    "sha256": "a" * 64,
                    "size": 1,
                    "archive": "zip",
                    "target": ".",
                }
            ],
            "delete": ["modules/game_wheel/"],
        }

    def test_manifest_merge_and_policy_fields(self):
        with tempfile.TemporaryDirectory() as temporary:
            one = self.manifest("core-windows")
            two = self.manifest("core-linux")
            two["components"][0]["platforms"] = ["linux"]
            paths = []
            for index, value in enumerate((one, two)):
                path = Path(temporary) / "{}.json".format(index)
                path.write_text(json.dumps(value), encoding="utf-8")
                paths.append(path)
            merged = merge_manifests(paths, Path(temporary) / "manifest.json")
            self.assertEqual(len(merged["components"]), 2)
            self.assertEqual(merged["protocol_version"], 1525)
            self.assertEqual(merged["asset_version"], "1525")
            self.assertEqual(merged["login_url"], "https://login.thappy.cl/login.php")
            self.assertEqual(merged["login_port"], 443)

    def test_size_baseline_gate(self):
        self.assertTrue(enforce_size_gate(110, None, 100)["within_regression_gate"])
        with self.assertRaisesRegex(ReleaseError, "more than 10%"):
            enforce_size_gate(111, None, 100)
        self.assertEqual(enforce_size_gate(100, None, None)["measured_budget"], 110)


class EvalSuiteTests(unittest.TestCase):
    def test_release_eval_fixture(self):
        completed = subprocess.run(
            [sys.executable, str(SCRIPTS / "run-client-release-evals.py")],
            cwd=SCRIPTS.parent,
            check=False,
            capture_output=True,
            text=True,
        )
        self.assertEqual(completed.returncode, 0, completed.stdout + completed.stderr)
        result = json.loads(completed.stdout)
        self.assertEqual(result["passed"], result["total"])
        self.assertEqual(result["total"], 14)


if __name__ == "__main__":
    unittest.main()
