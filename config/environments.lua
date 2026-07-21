local environments = {
    production = {
        configured = true,
        locked = true,
        channel = 'stable',
        loginUrl = 'https://login.thappy.cl/login.php',
        loginPort = 443,
        protocolVersion = 1525,
        httpLogin = false,
        useAuthenticator = false,
        websiteUrl = 'https://thappy.cl',
        supportUrl = 'https://thappy.cl',
        updateManifestUrl = 'https://github.com/ecantillano/otclient/releases/latest/download/manifest-stable.json',
        servers = {
            ['https://login.thappy.cl/login.php'] = {
                port = 443,
                protocol = 1525,
                httpLogin = false,
                useAuthenticator = false
            }
        }
    },

    -- Test and local builds are intentionally inert in source control. A dedicated
    -- build must inject a reviewed endpoint before selecting either environment.
    test = {
        configured = false,
        locked = true,
        channel = 'test',
        loginUrl = '',
        loginPort = 443,
        protocolVersion = 1525,
        httpLogin = false,
        useAuthenticator = false,
        websiteUrl = 'https://thappy.cl',
        supportUrl = 'https://thappy.cl',
        updateManifestUrl = '',
        servers = {}
    },

    localDevelopment = {
        configured = false,
        locked = true,
        channel = 'local',
        loginUrl = '',
        loginPort = 443,
        protocolVersion = 1525,
        httpLogin = false,
        useAuthenticator = false,
        websiteUrl = 'https://thappy.cl',
        supportUrl = 'https://thappy.cl',
        updateManifestUrl = '',
        servers = {}
    }
}

return environments
