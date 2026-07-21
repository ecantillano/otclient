local migration = {}

local migrationVersion = 1
local migrationKey = 'thappy-settings-migration'

-- Exact persisted nodes owned by the removed automation system. Normal player
-- options, keybinds, window state, audio, graphics and minimap nodes are untouched.
local removedBotKeys = {
    'Bot',
    'bot',
    'game_bot',
    'vBot',
    'vbot',
    'rvBot',
    'rvbot',
    'cavebot',
    'CaveBot',
    'targetbot',
    'TargetBot',
    'botserver',
    'default_configs',
    'macros',
    'profile'
}

local function copySafeLoginPreferences(serverSettings)
    local safe = {}
    if type(serverSettings) ~= 'table' then
        return safe
    end

    if type(serverSettings.account) == 'string' then
        safe.account = serverSettings.account
    end
    if type(serverSettings.password) == 'string' then
        safe.password = serverSettings.password
    end
    if type(serverSettings.autologin) == 'boolean' then
        safe.autologin = serverSettings.autologin
    end
    return safe
end

function migration.sanitizeProduction(settings, environment)
    if not environment or not environment.locked then
        return
    end

    for _, key in ipairs(removedBotKeys) do
        settings.remove(key)
    end

    local previousServers = settings.getNode('ServerList') or {}
    local persisted = copySafeLoginPreferences(previousServers[environment.loginUrl])
    persisted.port = environment.loginPort
    persisted.protocol = environment.protocolVersion
    persisted.httpLogin = environment.httpLogin
    persisted.useAuthenticator = environment.useAuthenticator

    settings.setNode('ServerList', {
        [environment.loginUrl] = persisted
    })
    settings.set('host', environment.loginUrl)
    settings.set('port', environment.loginPort)
    settings.set('client-version', environment.protocolVersion)
    settings.set('httpLogin', environment.httpLogin)
    settings.set(migrationKey, migrationVersion)
end

return migration
