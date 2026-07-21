-- First file executed when the application starts.

local Environments = dofile('/config/environments.lua')
local BuildConfig = dofile('/config/build_config.lua')
local RetroModules = dofile('/config/retro_modules.lua')
local SettingsMigration = dofile('/config/settings_migration.lua')

local ActiveEnvironment = Environments[BuildConfig.environment]
if not ActiveEnvironment or not ActiveEnvironment.configured then
    error(string.format("Thappy environment '%s' is not configured", tostring(BuildConfig.environment)))
end
if BuildConfig.channel ~= ActiveEnvironment.channel then
    error(string.format("Thappy build channel '%s' does not match environment channel '%s'",
        tostring(BuildConfig.channel), tostring(ActiveEnvironment.channel)))
end

ThappyBuild = {
    environment = BuildConfig.environment,
    channel = ActiveEnvironment.channel,
    loginUrl = ActiveEnvironment.loginUrl,
    loginPort = ActiveEnvironment.loginPort,
    protocolVersion = ActiveEnvironment.protocolVersion,
    httpLogin = ActiveEnvironment.httpLogin,
    useAuthenticator = ActiveEnvironment.useAuthenticator,
    websiteUrl = ActiveEnvironment.websiteUrl,
    supportUrl = ActiveEnvironment.supportUrl,
    updateManifestUrl = ActiveEnvironment.updateManifestUrl,
    clientVersion = BuildConfig.clientVersion,
    assetVersion = BuildConfig.assetVersion,
    buildCommit = BuildConfig.buildCommit,
    buildDate = BuildConfig.buildDate
}

Services = {
    websites = ActiveEnvironment.websiteUrl,
    support = ActiveEnvironment.supportUrl,
    updateManifest = ActiveEnvironment.updateManifestUrl,
    clientAssets = {
        -- Official releases never fetch third-party client data. Assets must be
        -- installed from an independently authorized distribution.
        enabled = false,
        strictManifestSha256 = true,
        allowRawFallbackHashMismatch = false
    }
}

-- The production profile resolves to exactly one immutable server. The login UI
-- reads this table without exposing host, port, protocol or authentication flags.
Servers_init = ActiveEnvironment.servers

g_app.setName('Thappy')
g_app.setCompactName('thappy')
g_app.setOrganizationName('Thappy')

g_app.hasUpdater = function()
    return Services.updater and Services.updater ~= '' and g_modules.getModule('updater')
end

g_logger.info('Operating system: ' .. g_platform.getOSName())
g_logger.info(g_app.getName() .. ' ' .. g_app.getVersion() .. ' rev ' .. g_app.getBuildRevision() .. ' (' ..
    g_app.getBuildCommit() .. ') built on ' .. g_app.getBuildDate() .. ' for arch ' .. g_app.getBuildArch())

if os.getenv('LOCAL_LUA_DEBUGGER_VSCODE') == '1' then
    require('lldebugger').start()
    g_logger.debug('Started LUA debugger.')
else
    g_logger.debug('LUA debugger not started (not launched with VSCode local-lua).')
end

if not g_resources.addSearchPath(g_resources.getWorkDir() .. 'data', true) then
    g_logger.fatal('Unable to add data directory to the search path.')
end
if not g_resources.addSearchPath(g_resources.getWorkDir() .. 'modules', true) then
    g_logger.fatal('Unable to add modules directory to the search path.')
end

g_html.addGlobalStyle('/data/styles/html.css')
g_html.addGlobalStyle('/data/styles/custom.css')

g_resources.setupUserWriteDir(('%s/'):format(g_app.getCompactName()))
g_resources.migrateLegacyUserData('otcr', 'otclient')
g_logger.setLogFile(g_resources.getWriteDir() .. '/thappy.log')
g_resources.searchAndAddPackages('/', '.otpkg', true)

local assetVersion = tonumber(ThappyBuild.assetVersion)
local requiredAssetCatalogs = {
    string.format('/data/things/%d/catalog-content.json', assetVersion),
    string.format('/data/sounds/%d/catalog-sound.json', assetVersion)
}
local missingAssetCatalogs = {}
for _, assetCatalog in ipairs(requiredAssetCatalogs) do
    if not g_resources.fileExists(assetCatalog) then
        table.insert(missingAssetCatalogs, assetCatalog)
    end
end
if #missingAssetCatalogs > 0 then
    g_logger.fatal(string.format(
        'Thappy cannot start because authorized client assets are missing: %s. ' ..
        'Install them at data/things/%d and data/sounds/%d; automatic downloads are disabled.',
        table.concat(missingAssetCatalogs, ', '), assetVersion, assetVersion))
end

g_configs.loadSettings('/config.otml')
SettingsMigration.sanitizeProduction(g_settings, ActiveEnvironment)
g_configs.saveSettings()

g_modules.discoverModules()

-- Explicit loading prevents optional upstream autoload flags from bypassing the
-- official visual profile.
g_modules.ensureModuleLoaded('corelib')
g_modules.ensureModuleLoaded('gamelib')
g_modules.ensureModuleLoaded('modulelib')
g_modules.ensureModuleLoaded('startup')

local function loadModuleList(moduleNames)
    for _, moduleName in ipairs(moduleNames) do
        g_modules.ensureModuleLoaded(moduleName)
    end
end

local function loadModules()
    g_modules.ensureModuleLoaded('client')
    loadModuleList(RetroModules.client)

    g_modules.ensureModuleLoaded('game_interface')
    loadModuleList(RetroModules.game)
end

if g_app.hasUpdater() then
    g_modules.ensureModuleLoaded('updater')
    return Updater.init(loadModules)
end

loadModules()
