controllerNpcTrader = Controller:new()
controllerNpcTrader.widthConsole = controllerNpcTrader.DEFAULT_CONSOLE_WIDTH
controllerNpcTrader.creatureName = ""
controllerNpcTrader.outfit = nil
controllerNpcTrader.buttons = {}
controllerNpcTrader.isTradeOpen = false
controllerNpcTrader.legacyMode = false
controllerNpcTrader.npcTalkConnected = false

function controllerNpcTrader:isLegacyMode()
    return self.legacyMode
end

function controllerNpcTrader:onInit()

end

function controllerNpcTrader:onGameStart()
    self.legacyMode = true
    if self:isLegacyMode() then
        self:legacy_init()
    end

    self:registerEvents(g_game, {
        onNpcChatWindow = function(data)
            onNpcChatWindow(data)
        end,
        onOpenNpcTrade = function(...)
            self:startEquippedImbuementsTracking()
            if self:isLegacyMode() then
                onOpenNpcTrade(...)
            else
                self:onOpenNpcTrade(...)
            end
        end,
        onPlayerGoods = function(money, items)
            if self:isLegacyMode() then
                onPlayerGoods(money, items)
            else
                self:onPlayerGoods(money, items)
            end
        end,
        onUpdateImbuementTracker = function(items)
            self:onUpdateEquippedImbuements(items)
        end,
        onNpcChatWindowClose = function()
            self:stopEquippedImbuementsTracking()
            if self:isLegacyMode() then
                self:legacy_hide()
            else
                self:onCloseNpcTrade()
            end
        end,
        onCloseNpcTrade = function()
            self:stopEquippedImbuementsTracking()
            if self:isLegacyMode() then
                self:legacy_hide()
            else
                self:onCloseNpcTrade()
            end
        end
    })
end

function controllerNpcTrader:connectNpcTalkEvent()
    if self:isLegacyMode() or self.npcTalkConnected then
        return
    end

    connect(g_game, {
        onTalk = onNpcTalk
    })
    self.npcTalkConnected = true
end

function controllerNpcTrader:disconnectNpcTalkEvent()
    if not self.npcTalkConnected then
        return
    end

    disconnect(g_game, {
        onTalk = onNpcTalk
    })
    self.npcTalkConnected = false
end

function controllerNpcTrader:onTerminate()
    if self:isLegacyMode() then
        self:legacy_terminate()
    else
        self:onCloseNpcTrade()
    end
end

function controllerNpcTrader:onGameEnd()
    if self:isLegacyMode() then
        self:legacy_hide()
    else
        self:onCloseNpcTrade()
    end
end

function controllerNpcTrader:onCloseNpcTrade()
    if self:isLegacyMode() then
        self:legacy_hide()
    else
        self:disconnectNpcTalkEvent()
        if controllerNpcTrader.ui and controllerNpcTrader.ui:isVisible() then
            controllerNpcTrader:unloadHtml()
        end
        controllerNpcTrader.isTradeOpen = false
        if controllerNpcTrader.sellAllWithDelayEvent then
            removeEvent(controllerNpcTrader.sellAllWithDelayEvent)
            controllerNpcTrader.sellAllWithDelayEvent = nil
        end
        -- Clean up state
        controllerNpcTrader.buyItems = {}
        controllerNpcTrader.sellItems = {}
        controllerNpcTrader.playerItems = {}
        controllerNpcTrader.playerMoney = nil
        controllerNpcTrader.selectedItem = nil
        controllerNpcTrader.tradeItems = {}
        controllerNpcTrader.currentList = {}
        controllerNpcTrader.allTradeItems = {}
    end
end
