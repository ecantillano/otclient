-- to-do
-- change to ItemsDatabase.setTier(UIitem) to UIitem:setTier()
ItemsDatabase = {}

ItemsDatabase.rarityColors = {
    ["yellow"] = TextColors.yellow,
    ["purple"] = TextColors.purple,
    ["blue"] = TextColors.blue,
    ["green"] = TextColors.green,
    ["grey"] = TextColors.grey,
}

-- Per-instance Thappy rarity colors. These are intentionally separate from
-- the legacy mean-price frames above and from official Exaltation Forge tiers.
ItemsDatabase.thappyRarityColors = {
    [1] = TextColors.grey,
    [2] = "#4388ff",
    [3] = "#b260ff",
    [4] = "#ff9f2f",
    [5] = "#e75cff",
}

ItemsDatabase.thappyBonusColor = "#54d66b"

local thappyRarityNames = {
    [1] = "Común",
    [2] = "Raro",
    [3] = "Épico",
    [4] = "Legendario",
    [5] = "Místico",
}

local thappyBonusLabels = {
    "Vida máxima",
    "Mana máximo",
    "Regeneración de vida",
    "Regeneración de mana",
    "Velocidad",
    "Protección física",
    "Protección de energía",
    "Protección de tierra",
    "Protección de fuego",
    "Protección de hielo",
    "Protección sagrada",
    "Protección de muerte",
    "Sword Fighting",
    "Axe Fighting",
    "Club Fighting",
    "Distance Fighting",
    "Shielding",
    "Magic Level",
}

function ItemsDatabase.setColorRarityMessage(text)
    -- Colored-text markup has no escaping. Preserve arbitrary Look text
    -- literally instead of interpreting braces supplied by another server.
    if text:find("{", 1, true) or text:find("}", 1, true) then
        return text, false
    end

    local lines = {}
    local tier
    local rarityTier
    for line in (text .. "\n"):gmatch("(.-)\n") do
        table.insert(lines, line)
        local exactTier = tonumber(line:match("^Tier ([1-5])$"))
        if exactTier then
            tier = exactTier
        end
        for candidateTier, rarityName in pairs(thappyRarityNames) do
            if line == "Rareza: " .. rarityName then
                rarityTier = candidateTier
                break
            end
        end
    end
    if not tier or rarityTier ~= tier then
        return text, false
    end

    local result = {}
    for _, line in ipairs(lines) do
        local color = TextColors.white
        local isBonus = false
        for _, label in ipairs(thappyBonusLabels) do
            if line:find(label .. " +", 1, true) == 1 then
                isBonus = true
                break
            end
        end
        if isBonus then
            color = ItemsDatabase.thappyBonusColor
        elseif line == "Tier " .. tier or line == "Rareza: " .. thappyRarityNames[tier] or
            line:find(thappyRarityNames[tier], 1, true) then
            color = ItemsDatabase.thappyRarityColors[tier]
        end
        table.insert(result, line == "" and "" or "{" .. line .. ", " .. color .. "}")
    end
    return table.concat(result, "\n"), true
end

local function getColorForValue(value)
    if value >= 1000000 then
        return "yellow"
    elseif value >= 100000 then
        return "purple"
    elseif value >= 10000 then
        return "blue"
    elseif value >= 1000 then
        return "green"
    elseif value >= 50 then
        return "grey"
    else
        return "white"
    end
end

local function clipfunction(value)
    if value >= 1000000 then
        return "128 0 32 32"
    elseif value >= 100000 then
        return "96 0 32 32"
    elseif value >= 10000 then
        return "64 0 32 32"
    elseif value >= 1000 then
        return "32 0 32 32"
    elseif value >= 50 then
        return "0 0 32 32"
    end
    return ""
end

function ItemsDatabase.getClipAndImagePath(item)
    if not item then
        return nil, nil, nil
    end

    local frameOption = modules.client_options.getOption('framesRarity')
    if frameOption == "none" then
        return nil, nil, nil
    end
    local imagePath = '/images/ui/item'
    local clip = nil

    if type(item) == "number" then
        item = g_things.getThingType(item, ThingCategoryItem)
    end

    if not item then
        return nil, nil, nil
    end

    if item then
        local price = type(item) == "number" and item or (item and item:getMeanPrice()) or 0
        local itemRarity = getColorForValue(price)
        if itemRarity then
            clip = clipfunction(price)
            if clip ~= "" then
                if frameOption == "frames" then
                    imagePath = "/images/ui/rarity_frames"
                elseif frameOption == "corners" then
                    imagePath = "/images/ui/containerslot-coloredges"
                end
            else
                clip = nil
            end
        end
    end

    local clipObject = nil
    if clip then
        local x, y, w, h = clip:match("(%d+) (%d+) (%d+) (%d+)")
        clipObject = { x = tonumber(x), y = tonumber(y), width = tonumber(w), height = tonumber(h) }
    end

    return clip, imagePath, clipObject
end

function ItemsDatabase.setRarityItem(widget, item, style)
    if not g_game.getFeature(GameColorizedLootValue) or not widget then
        return
    end

    local clip, imagePath = ItemsDatabase.getClipAndImagePath(item)

    if not imagePath then
        return
    end

    widget:setImageClip(clip)
    widget:setImageSource(imagePath)
    if style then
        widget:setStyle(style)
    end
end

function ItemsDatabase.getColorForRarity(rarity)
    return ItemsDatabase.rarityColors[rarity] or TextColors.white
end

function ItemsDatabase.setColorLootMessage(text)
    local function coloringLootName(match)
        local id, itemName = match:match("(%d+)|(.+)")
        if not id or not itemName then
            -- If pattern doesn't match itemId|itemName format, return the original match with braces
            return "{" .. match .. "}"
        end

        local itemId = tonumber(id)
        if not itemId then
            return itemName or match
        end

        local thingType = g_things.getThingType(itemId, ThingCategoryItem)
        if not thingType then
            return itemName
        end

        local itemInfo = thingType:getMeanPrice()
        if itemInfo then
            local color = ItemsDatabase.getColorForRarity(getColorForValue(itemInfo))
            return "{" .. itemName .. ", " .. color .. "}"
        else
            return itemName
        end
    end
    return text:gsub("{(.-)}", coloringLootName)
end

function ItemsDatabase.getTierClip(tier)
    local xOffset = (math.min(math.max(tier, 1), 10) - 1) * 9
    return {
        x = xOffset,
        y = 0,
        width = 10,
        height = 9
    }
end

function ItemsDatabase.setTier(widget, item, isSmall)
    if not g_game.getFeature(GameThingUpgradeClassification) or not widget or not widget.tier then
        return
    end
    if isSmall == nil then
        isSmall = true
    end
    local tier = type(item) == "number" and item or (item and item:getTier()) or 0
    if tier <= 0 then
        widget.tier:setVisible(false)
        return
    end
    local config
    if isSmall then
        local normalizedTier = math.min(math.max(tier, 1), 10)
        config = {
            xOffset = (normalizedTier - 1) * 9,
            width = 10,
            height = 9,
            size = "10 9",
            source = '/images/inventory/tiers-strip'
        }
    else
        local normalizedTier = math.min(math.max(tier, 1), 18)
        local xOffset = (normalizedTier - 1) * 18 + 1
        config = {
            xOffset = xOffset,
            width = 18,
            height = 16,
            size = "18 16",
            source = '/images/inventory/tiers-strip-big'
        }
    end

    widget.tier:setImageClip({
        x = config.xOffset,
        y = 0,
        width = config.width,
        height = config.height
    })
    widget.tier:setSize(config.size)
    widget.tier:setImageSource(config.source)
    widget.tier:setImageSize(config.size)
    widget.tier:setVisible(true)
end
