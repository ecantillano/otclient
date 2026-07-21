#!/usr/bin/env python3
"""Deterministic release-readiness eval for the Thappy visual/runtime contract."""

from __future__ import annotations

import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def contains(path: str, *needles: str) -> bool:
    text = (ROOT / path).read_text(encoding="utf-8")
    return all(needle in text for needle in needles)


checks = {
    "canonical-production": contains(
        "config/environments.lua",
        "https://login.thappy.cl/login.php",
        "loginPort = 443",
        "protocolVersion = 1525",
        "httpLogin = false",
        "useAuthenticator = false",
    ),
    "immutable-login": contains(
        "modules/client_entergame/entergame.lua",
        "G.host = ThappyBuild.loginUrl",
        "G.port = ThappyBuild.loginPort",
        "local clientVersion = ThappyBuild.protocolVersion",
    ),
    "authorized-assets-only": contains(
        "init.lua",
        "enabled = false",
        "/data/things/%d/catalog-content.json",
        "/data/sounds/%d/catalog-sound.json",
        "automatic downloads are disabled",
    )
    and "repository =" not in (ROOT / "init.lua").read_text(encoding="utf-8"),
    "retro-core": contains(
        "config/retro_modules.lua",
        "'game_inventory'",
        "'game_battle'",
        "'game_console'",
        "'game_minimap'",
        "'game_hotkeys'",
        "'game_npctrade'",
    ),
    "retro-actions-only": "game_cyclopedia" not in (
        (ROOT / "modules/game_minimap/minimap.lua").read_text(encoding="utf-8").split(
            "function openCyclopediaMap()", 1
        )[1].split("\nend", 1)[0]
    )
    and "modules.game_store" not in (ROOT / "modules/game_skills/skills.otui").read_text(encoding="utf-8")
    and "/game_cyclopedia/" not in (ROOT / "modules/game_skills/skills.otui").read_text(encoding="utf-8")
    and "modules.game_blessing" not in (ROOT / "modules/game_inventory/inventory.otui").read_text(encoding="utf-8")
    and "stashStowItem" not in (ROOT / "modules/game_interface/gameinterface.lua").read_text(encoding="utf-8"),
    "bot-removed": not (ROOT / "mods/game_bot").exists()
    and not (ROOT / "mods/client_profiles").exists(),
    "safe-migration": contains(
        "src/framework/core/resourcemanager.cpp",
        "!m_userDirOverride.empty()",
        "std::filesystem::copy_options::none",
        ".thappy-legacy-migration-v1",
        '"cavebot"',
        '"targetbot"',
    ),
    "client-update-lock": contains(
        "src/main.cpp",
        'stateDir / "launcher.lock"',
        "LOCK_EX | LOCK_NB",
        "LOCKFILE_EXCLUSIVE_LOCK | LOCKFILE_FAIL_IMMEDIATELY",
        'std::getenv("THAPPY_MANAGED_LAUNCH")',
    )
    and contains("launcher/runner.go", '"THAPPY_MANAGED_LAUNCH=1"'),
}

passed = sum(checks.values())
score = passed / len(checks)
for name, result in checks.items():
    print(f"{'PASS' if result else 'FAIL'} {name}")
print(f"score={score:.2f} threshold=1.00")
sys.exit(0 if score == 1.0 else 1)
