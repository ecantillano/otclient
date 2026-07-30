-- Real Lua runtime eval for the Thappy Look formatter.
TextColors = {
    grey = "#AAAAAA",
    white = "#ffffff",
    green = "#00ff00",
}

dofile(arg[1] or "modules/gamelib/items.lua")

local tierColors = {
    [1] = "#AAAAAA",
    [2] = "#4388ff",
    [3] = "#b260ff",
    [4] = "#ff9f2f",
    [5] = "#e75cff",
}
local rarityNames = { "Común", "Raro", "Épico", "Legendario", "Místico" }

for tier = 1, 5 do
    local look = table.concat({
        "You see an " .. rarityNames[tier] .. " plate armor.",
        "Tier " .. tier,
        "Armor: 10",
        "",
        "Rareza: " .. rarityNames[tier],
        "Vida máxima +8",
    }, "\n")
    local colored, isRarity = ItemsDatabase.setColorRarityMessage(look)
    assert(isRarity)
    assert(colored:find("{Tier " .. tier .. ", " .. tierColors[tier] .. "}", 1, true))
    assert(colored:find("{Armor: 10, #ffffff}", 1, true))
    assert(not colored:find("{, ", 1, true))
    assert(colored:find("{Vida máxima +8, #54d66b}", 1, true))
end

local normal = "You see a plate armor."
local normalOutput, normalRarity = ItemsDatabase.setColorRarityMessage(normal)
assert(normalOutput == normal and not normalRarity)

local forgeTier = "You see an item.\nTier 10\nRareza: Común"
local forgeOutput, forgeRarity = ItemsDatabase.setColorRarityMessage(forgeTier)
assert(forgeOutput == forgeTier and not forgeRarity)

local missingMarker = "You see an item.\nTier 1"
local markerOutput, markerRarity = ItemsDatabase.setColorRarityMessage(missingMarker)
assert(markerOutput == missingMarker and not markerRarity)

local braces = "You see {literal, #ff0000}.\nTier 1\nRareza: Común"
local bracesOutput, bracesRarity = ItemsDatabase.setColorRarityMessage(braces)
assert(bracesOutput == braces and not bracesRarity)

print("THAPPY RARITY CLIENT LUA RUNTIME EVAL PASSED")
