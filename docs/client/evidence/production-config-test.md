# Evidencia de configuración production

Fecha: 2026-07-21

Los paquetes Windows y Linux producidos por el run CI 29858920649 pasaron
`scripts/test-production-config.py` y la inspección semántica del auditor.

Configuración encontrada en ambos artefactos:

```lua
Servers_init = {
    ["https://login.thappy.cl/login.php"] = {
        port = 443,
        protocol = 1525,
        httpLogin = false,
        useAuthenticator = false
    }
}
```

El perfil es `production`, canal `stable`, cliente `0.1.0`, assets `1525`. El
host, puerto y protocolo no son editables desde el login y la configuración
persistida no puede reemplazarlos. La migración conserva opciones, hotkeys y
minimap, pero purga servidores heredados y claves exactas del BOT.

Gates negativos verificados:

- sin `192.168.50.248`, localhost, loopback ni rangos RFC1918;
- sin dominios test o staging;
- sin protocolo production distinto de 1525;
- sin `httpLogin=true` ni `useAuthenticator=true`;
- sin selector público de host, puerto o protocolo.

El 2026-07-21 DNS y TLS respondieron en `login.thappy.cl:443`. OpenSSL obtuvo
`ssl_verify_result=0`, SAN `DNS:login.thappy.cl` y una cadena Let's Encrypt
vigente. Un GET anónimo devolvió 404, resultado esperado porque la configuración
canónica usa login nativo con `httpLogin=false`.

No se enviaron credenciales. Character list y entrada al game server no fueron
probados porque no se proporcionó una cuenta de test no personal ni assets 1525
autorizados.
