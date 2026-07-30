#!/usr/bin/env python3
"""Fast client contract eval for Thappy rarity presentation."""

from pathlib import Path
import re


ROOT = Path(__file__).resolve().parents[2]


def main() -> None:
    items = (ROOT / "modules/gamelib/items.lua").read_text(encoding="utf-8")
    textmessage = (ROOT / "modules/game_textmessage/textmessage.lua").read_text(encoding="utf-8")
    interface = (ROOT / "modules/game_interface/interface.otmod").read_text(encoding="utf-8")
    visual = (ROOT / "tests/evals/thappy_rarity_visual_reference.svg").read_text(encoding="utf-8")

    colors = dict(
        (int(tier), color)
        for tier, color in re.findall(r'^\s*\[(\d)\]\s*=\s*"(#[0-9a-fA-F]{6})",?$', items, re.M)
    )
    assert colors[2] == "#4388ff"
    assert colors[3] == "#b260ff"
    assert colors[4] == "#ff9f2f"
    assert colors[5] == "#e75cff"
    assert len(set(colors.values())) == 4
    assert 'ItemsDatabase.thappyBonusColor = "#54d66b"' in items
    assert 'line:match("^Tier ([1-5])$")' in items
    assert 'line == "Rareza: " .. rarityName' in items
    assert 'line:find(label .. " +", 1, true) == 1' in items
    assert 'return text, false' in items
    assert "setColorRarityMessage(text)" in textmessage
    assert "[MessageModes.Look] = MessageSettings.thappyLook" in textmessage
    assert "thappyLook = {\n        color = TextColors.green" in textmessage
    assert "if isThappyRarity then" in textmessage
    assert "label:setText(text)" in textmessage
    assert (ROOT / "tests/evals/thappy_rarity_ui_runtime.lua").is_file()
    assert "game_forge" not in interface
    assert "GameItemTooltipV8" not in items
    for color in ("#AAAAAA", "#4388ff", "#b260ff", "#ff9f2f", "#e75cff", "#54d66b"):
        assert color in visual
    assert "Referencia visual determinística, no captura runtime" in visual
    print("THAPPY RARITY CLIENT UI EVAL PASSED")


if __name__ == "__main__":
    main()
