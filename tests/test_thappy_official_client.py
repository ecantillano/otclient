#!/usr/bin/env python3
"""Fast deterministic gates for the official Thappy runtime profile."""

from __future__ import annotations

import re
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


def read(relative: str) -> str:
    return (ROOT / relative).read_text(encoding="utf-8")


def lua_string_list(source: str, key: str) -> set[str]:
    match = re.search(rf"\b{re.escape(key)}\s*=\s*\{{(.*?)\n\s*\}}", source, re.S)
    if not match:
        raise AssertionError(f"Lua list {key!r} was not found")
    return set(re.findall(r"['\"]([^'\"]+)['\"]", match.group(1)))


class ProductionConfigTests(unittest.TestCase):
    def test_production_connection_is_exact_and_isolated(self) -> None:
        environments = read("config/environments.lua")
        production = environments.split("    test = {", 1)[0]
        self.assertEqual(production.count("https://login.thappy.cl/login.php"), 2)
        for expected in (
            "loginPort = 443",
            "protocolVersion = 1525",
            "httpLogin = false",
            "useAuthenticator = false",
            "port = 443",
            "protocol = 1525",
            "locked = true",
        ):
            self.assertIn(expected, production)
        for forbidden in ("localhost", "127.0.0.1", "192.168.", "10.0.", "staging"):
            self.assertNotIn(forbidden, production.lower())

    def test_source_metadata_uses_release_sources(self) -> None:
        build = read("config/build_config.lua")
        self.assertIn("environment = 'production'", build)
        self.assertIn("channel = 'stable'", build)
        self.assertIn(f"clientVersion = '{read('VERSION').strip()}'", build)
        self.assertIn(f"assetVersion = {read('ASSET_VERSION').strip()}", build)

    def test_init_locks_server_and_security_defaults(self) -> None:
        init = read("init.lua")
        self.assertIn("Servers_init = ActiveEnvironment.servers", init)
        self.assertIn("enabled = false", init)
        self.assertIn("strictManifestSha256 = true", init)
        self.assertIn("allowRawFallbackHashMismatch = false", init)
        self.assertNotIn("repository =", init)
        self.assertNotIn("dudantas/tibia-client", init)
        self.assertNotIn("autoLoadModules", init)
        self.assertNotIn("client_mods", init)
        self.assertNotRegex(init, r"addSearchPath\([^\n]*mods")

    def test_missing_authorized_assets_stop_startup_without_fetching(self) -> None:
        init = read("init.lua")
        self.assertIn("/data/things/%d/catalog-content.json", init)
        self.assertIn("/data/sounds/%d/catalog-sound.json", init)
        self.assertIn("authorized client assets are missing", init)
        self.assertIn("automatic downloads are disabled", init)
        self.assertLess(init.index("missingAssetCatalogs"), init.index("g_modules.discoverModules()"))

    def test_login_ignores_persisted_connection_values(self) -> None:
        login = read("modules/client_entergame/entergame.lua")
        self.assertIn("G.host = ThappyBuild.loginUrl", login)
        self.assertIn("G.port = ThappyBuild.loginPort", login)
        self.assertIn("local clientVersion = ThappyBuild.protocolVersion", login)
        self.assertIn("local httpLogin = ThappyBuild.httpLogin", login)
        self.assertNotIn("loadServerListModule", login)
        ui = read("modules/client_entergame/entergame.otui")
        self.assertNotIn("@onClick: EnterGame.showServerList()", ui)


class PersistenceTests(unittest.TestCase):
    def test_credentials_have_one_encrypt_and_one_decrypt_boundary(self) -> None:
        login = read("modules/client_entergame/entergame.lua")
        self.assertIn("server.account = g_crypt.encrypt(account or '')", login)
        self.assertIn("server.password = g_crypt.encrypt(password or '')", login)
        account_setter = re.search(
            r"function EnterGame\.setAccountName\(account\)(.*?)\nend", login, re.S
        )
        password_setter = re.search(
            r"function EnterGame\.setPassword\(password\)(.*?)\nend", login, re.S
        )
        self.assertIsNotNone(account_setter)
        self.assertIsNotNone(password_setter)
        self.assertEqual(account_setter.group(1).count("safeDecrypt("), 1)
        self.assertEqual(password_setter.group(1).count("safeDecrypt("), 1)
        self.assertIn("EnterGame.setAccountName(serverData.account)", login)
        self.assertIn("EnterGame.setPassword(serverData.password)", login)

    def test_legacy_migration_is_allowlisted_and_override_safe(self) -> None:
        resource_manager = read("src/framework/core/resourcemanager.cpp")
        self.assertIn("!m_userDirOverride.empty()", resource_manager)
        self.assertIn('path == "config.otml"', resource_manager)
        self.assertIn('first == "controls"', resource_manager)
        self.assertIn('first == "screenshots"', resource_manager)
        self.assertIn('filename.starts_with("minimap")', resource_manager)
        self.assertIn("std::filesystem::copy_options::none", resource_manager)
        self.assertIn(".thappy-legacy-migration-v1", resource_manager)
        for forbidden in ("game_bot", "vbot", "cavebot", "targetbot", "macro"):
            self.assertIn(f'"{forbidden}"', resource_manager)

        migration = read("config/settings_migration.lua")
        self.assertIn("settings.setNode('ServerList', {", migration)
        self.assertIn("[environment.loginUrl] = persisted", migration)
        for preserved in ("game_hotkeys", "Minimap", "window-size", "enableAudio"):
            self.assertNotRegex(migration, rf"settings\.remove\(['\"]{preserved}")


class RuntimeProfileTests(unittest.TestCase):
    REQUIRED = {
        "game_inventory",
        "game_containers",
        "game_battle",
        "game_console",
        "game_minimap",
        "game_skills",
        "game_viplist",
        "game_hotkeys",
        "game_npctrade",
        "game_playertrade",
    }
    MODERN = {
        "client_bottommenu",
        "game_actionbar",
        "game_analyser",
        "game_cyclopedia",
        "game_forge",
        "game_healthcircle",
        "game_market",
        "game_prey",
        "game_proficiency",
        "game_quickloot",
        "game_rewardwall",
        "game_shop",
        "game_stash",
        "game_store",
        "game_taskboard",
        "game_wheel",
    }

    def test_retro_allowlist_preserves_classic_play(self) -> None:
        profile = read("config/retro_modules.lua")
        active = lua_string_list(profile, "client") | lua_string_list(profile, "game")
        disabled = lua_string_list(profile, "disabledVisual")
        self.assertTrue(self.REQUIRED <= active)
        self.assertTrue(self.MODERN <= disabled)
        self.assertFalse(active & disabled)
        self.assertFalse(active & self.MODERN)

    def test_explicit_module_graph_has_resolvable_safe_dependencies(self) -> None:
        profile = read("config/retro_modules.lua")
        active = {
            "corelib",
            "gamelib",
            "modulelib",
            "startup",
            "client",
            "game_interface",
        } | lua_string_list(profile, "client") | lua_string_list(profile, "game")
        disabled = lua_string_list(profile, "disabledVisual")
        discovered: dict[str, str] = {}
        for module_file in (ROOT / "modules").glob("*/*.otmod"):
            source = module_file.read_text(encoding="utf-8")
            name = re.search(r"(?m)^\s*name:\s*([^\s#]+)", source)
            if name:
                discovered[name.group(1)] = source

        dependency_pattern = re.compile(
            r"(?ms)^\s*dependencies:\s*(?:\[([^\]]*)\]|\n((?:\s+-\s*[^\n]+\n?)+))"
        )
        closure = set(active)
        queue = list(active)
        while queue:
            module_name = queue.pop()
            self.assertIn(module_name, discovered, f"missing module {module_name}")
            match = dependency_pattern.search(discovered[module_name])
            if not match:
                continue
            dependencies = re.findall(r"[A-Za-z0-9_]+", match.group(1) or match.group(2))
            for dependency in dependencies:
                self.assertIn(dependency, discovered, f"missing dependency {dependency}")
                self.assertNotIn(dependency, disabled, f"disabled visual dependency {dependency}")
                if dependency not in closure:
                    closure.add(dependency)
                    queue.append(dependency)

        self.assertTrue({"client_assets", "game_features", "game_things"} <= closure)

    def test_bot_and_bot_profile_are_deleted(self) -> None:
        self.assertFalse((ROOT / "mods/game_bot").exists())
        self.assertFalse((ROOT / "mods/client_profiles").exists())
        loader = read("mods/client_mods/mods.otmod").lower()
        self.assertNotIn("game_bot", loader)
        self.assertNotIn("client_profiles", loader)

    def test_protocol_parsers_remain_compiled(self) -> None:
        parser = read("src/client/protocolgameparse.cpp")
        self.assertIn("parseStore", parser)
        self.assertIn("parseCyclopedia", parser)
        self.assertIn("parseTask", parser)

    def test_allowed_retro_modules_do_not_expose_disabled_modern_actions(self) -> None:
        minimap = read("modules/game_minimap/minimap.lua")
        open_map = re.search(r"function openCyclopediaMap\(\)(.*?)\nend", minimap, re.S)
        self.assertIsNotNone(open_map)
        self.assertIn("fullscreen()", open_map.group(1))
        self.assertNotIn("game_cyclopedia", open_map.group(1))

        skills_ui = read("modules/game_skills/skills.otui")
        self.assertNotIn("modules.game_store", skills_ui)
        self.assertNotIn("sendRequestStorePremiumBoost", skills_ui)
        self.assertNotIn("/game_cyclopedia/", skills_ui)
        skills_lua = read("modules/game_skills/skills.lua")
        self.assertNotIn("combatIdToWidgetId", skills_lua)

        inventory_ui = read("modules/game_inventory/inventory.otui")
        self.assertNotIn("modules.game_blessing", inventory_ui)
        self.assertNotRegex(inventory_ui, r"(?m)^\s*id:\s*blessings\s*$")

        interface = read("modules/game_interface/gameinterface.lua")
        self.assertNotIn('tr("Stow")', interface)
        self.assertNotIn("stashStowItem", interface)
        self.assertNotIn("styles/countStashWindow", interface)

        outfit = read("modules/game_outfit/outfit.lua")
        self.assertIn("local function getAttachedEffectsModule()", outfit)
        self.assertIn("wings = effectModule and wingsList or {}", outfit)
        for line in outfit.splitlines():
            if "modules.game_attachedeffects" in line:
                self.assertIn("local effectModule =", line)

    def test_application_identity_and_log_paths(self) -> None:
        init = read("init.lua")
        self.assertIn("g_app.setName('Thappy')", init)
        self.assertIn("g_app.setCompactName('thappy')", init)
        self.assertIn("g_resources.getWriteDir() .. '/thappy.log'", init)
        cmake = read("src/CMakeLists.txt")
        self.assertIn('OUTPUT_NAME "Thappy"', cmake)
        self.assertIn('OUTPUT_NAME "thappy"', cmake)
        self.assertIn("configure_file(", cmake)
        self.assertIn('file(READ "${CMAKE_SOURCE_DIR}/VERSION" VERSION)', cmake)

    def test_direct_client_holds_launcher_install_lock(self) -> None:
        main = read("src/main.cpp")
        runner = read("launcher/runner.go")
        self.assertIn('stateDir / "launcher.lock"', main)
        self.assertIn("LOCK_EX | LOCK_NB", main)
        self.assertIn("LOCKFILE_EXCLUSIVE_LOCK | LOCKFILE_FAIL_IMMEDIATELY", main)
        self.assertIn('std::getenv("THAPPY_MANAGED_LAUNCH")', main)
        self.assertIn('"THAPPY_MANAGED_LAUNCH=1"', runner)


if __name__ == "__main__":
    unittest.main()
