-- Official source defaults. Release automation may replace metadata values, but
-- production remains the default and an environment is never selected at runtime.
return {
    environment = 'production',
    channel = 'stable',
    clientVersion = '0.1.0',
    assetVersion = 1525,
    buildCommit = 'unknown',
    buildDate = 'unknown'
}
