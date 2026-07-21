#!/usr/bin/env python3
"""Run deterministic release-policy eval cases used by CI and nightly jobs."""

import json
import sys
from pathlib import Path

from client_release import ArchiveEntry, ReleaseError, audit_entries, enforce_size_gate, validate_manifest


ROOT = Path(__file__).resolve().parents[1]
FIXTURE = ROOT / "evals" / "client-release-cases.json"


def entry(name: str, data=b"safe") -> ArchiveEntry:
    if isinstance(data, str):
        data = data.encode("utf-8")
    return ArchiveEntry(name, len(data), len(data), 0o644, data)


def production_entries(protocol: int = 1525):
    config = """
return {
  production = {
    loginUrl = 'https://login.thappy.cl/login.php', loginPort = 443,
    protocolVersion = 1525, httpLogin = false, useAuthenticator = false,
    updateManifestUrl = 'https://github.com/ecantillano/otclient/releases/latest/download/manifest-stable.json',
    servers = { ['https://login.thappy.cl/login.php'] = {
      port = 443, protocol = %d, httpLogin = false, useAuthenticator = false
    } }
  }
}
""" % protocol
    return [
        entry("Thappy.exe"),
        entry("ThappyLauncher.exe"),
        entry("LICENSE"),
        entry("THIRD_PARTY_NOTICES.md"),
        entry("licenses/OTClient-MIT.txt"),
        entry("config/environments.lua", config),
        entry("data/styles/base.otui"),
        entry("modules/game_interface/interface.lua"),
    ]


def audit_outcome(entries, production=False):
    result = audit_entries(
        entries,
        "eval.zip",
        compressed_size=1,
        require_layout=production,
        require_production_config=production,
    )
    return "pass" if result.ok else "fail"


def evaluate(name: str) -> str:
    if name == "valid_production_package":
        return audit_outcome(production_entries(), production=True)
    if name == "locale_boton_is_not_bot":
        return audit_outcome([entry("data/locales/es.lua", "botón botón botão")])
    if name == "purge_only_settings_migration":
        return audit_outcome(
            [
                entry(
                    "config/settings_migration.lua",
                    """local removedBotKeys = {'game_bot', 'vBot', 'cavebot', 'targetbot', 'default_configs'}
for _, key in ipairs(removedBotKeys) do
  settings.remove(key)
end
""",
                )
            ]
        )
    if name == "launcher_config_purge_only":
        return audit_outcome(
            [entry("launcher-config.json", '{"delete_allowlist":["mods/game_bot/"],"channels":{}}')]
        )
    if name == "game_bot_module_path":
        return audit_outcome([entry("modules/game_bot/bot.lua")])
    if name == "bot_button_reference":
        return audit_outcome([entry("modules/game_interface/ui.lua", "modules.game_bot.showBotButton()")])
    if name == "modern_ui_module":
        return audit_outcome([entry("modules/game_wheel/wheel.lua")])
    if name == "pdb_or_elf_debug_symbols":
        return audit_outcome([entry("thappy", b"\x7fELF.debug_info")])
    if name == "duplicate_runtime_library":
        return audit_outcome([entry("foo.dll", b"one"), entry("plugins/FOO.DLL", b"two")])
    if name == "private_production_endpoint":
        return audit_outcome([entry("config/environment.lua", "login = 'http://127.0.0.1/login'")])
    if name == "wrong_production_protocol":
        return audit_outcome(production_entries(protocol=1098), production=True)
    if name == "source_or_build_tree":
        return audit_outcome([entry("build/CMakeFiles/client.obj")])
    if name == "size_regression_over_ten_percent":
        try:
            enforce_size_gate(111, None, 100)
        except ReleaseError:
            return "fail"
        return "pass"
    if name == "broad_delete_root":
        manifest = {
            "schema_version": 1,
            "channel": "stable",
            "version": "0.1.0",
            "protocol_version": 1525,
            "asset_version": "1525",
            "release_notes_url": "https://github.com/ecantillano/otclient/releases/tag/client-v0.1.0",
            "mandatory": False,
            "minimum_launcher_version": "0.1.0",
            "environment": "production",
            "login_url": "https://login.thappy.cl/login.php",
            "login_port": 443,
            "http_login": False,
            "use_authenticator": False,
            "components": [{
                "name": "core-windows", "version": "0.1.0", "platforms": ["windows"],
                "url": "https://github.com/ecantillano/otclient/releases/download/client-v0.1.0/core.zip",
                "sha256": "a" * 64, "size": 1, "archive": "zip", "target": ".",
            }],
            "delete": ["modules/"],
        }
        try:
            validate_manifest(manifest)
        except ReleaseError:
            return "fail"
        return "pass"
    if name == "release_docs_match_workflow":
        workflow = (ROOT / ".github" / "workflows" / "client-release.yml").read_text(encoding="utf-8")
        documentation = (ROOT / "docs" / "client" / "09-release-process.md").read_text(
            encoding="utf-8"
        )
        inputs = (
            "version",
            "asset_version",
            "channel",
            "test_login_url",
            "test_login_port",
            "test_website_url",
            "test_support_url",
            "test_manifest_url",
            "mandatory",
        )
        if "publish_mode=" in documentation:
            return "fail"
        if any("{}:".format(item) not in workflow for item in inputs):
            return "fail"
        if any("-f {}=".format(item) not in documentation for item in inputs):
            return "fail"
        return "pass"
    if name == "workflow_retries_cmake_configure":
        ci = (ROOT / ".github" / "workflows" / "client-ci.yml").read_text(encoding="utf-8")
        release = (ROOT / ".github" / "workflows" / "client-release.yml").read_text(
            encoding="utf-8"
        )
        invocation = "scripts/retry-cmake-configure.sh cmake --preset"
        if ci.count(invocation) < 2 or invocation not in release:
            return "fail"
        return "pass"
    raise ReleaseError("unknown eval case: {}".format(name))


def main() -> int:
    fixture = json.loads(FIXTURE.read_text(encoding="utf-8"))
    results = []
    for case in fixture.get("cases", []):
        actual = evaluate(case["name"])
        results.append(
            {
                "name": case["name"],
                "expected": case["expected"],
                "actual": actual,
                "ok": actual == case["expected"],
            }
        )
    passed = sum(result["ok"] for result in results)
    report = {
        "schema_version": fixture.get("schema_version"),
        "passed": passed,
        "total": len(results),
        "cases": results,
    }
    print(json.dumps(report, indent=2, sort_keys=True))
    return 0 if passed == len(results) and results else 1


if __name__ == "__main__":
    sys.exit(main())
