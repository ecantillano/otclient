# Ambientes del cliente

## Producción

El artefacto oficial usa `production` por defecto. La fuente de verdad vive en
`config/environments.lua` y es exactamente:

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

El login no lee host, puerto, protocolo ni flags de autenticación desde el
estado persistido. `modules/client_entergame/entergame.lua` toma esos valores de
`ThappyBuild`, oculta los controles correspondientes y reemplaza `ServerList`
por la entrada oficial al iniciar.

HTTPS no cambia `httpLogin`: el valor confirmado sigue siendo `false`.

## Test y desarrollo local

`config/environments.lua` mantiene `test` y `localDevelopment` sin endpoint y
con `configured = false`. No pueden seleccionarse por accidente desde un
checkout limpio. El empaquetador genera una copia temporal de la configuración
cuando recibe un ambiente no productivo y exige por línea de comandos todos
los valores de endpoint. La fuente del repositorio no se modifica.

| Ambiente | Canal | Selección | Endpoint en Git |
|---|---|---|---|
| `production` | `stable` | valor por defecto del build oficial | endpoint canónico |
| `test` | `test` | build dedicado de CI/empaquetador | ninguno |
| `localDevelopment` | `local` | build local explícito | ninguno |

No existe selector de ambiente, host, puerto o protocolo en la interfaz del
jugador.

## Metadata verificable del artefacto

Cada paquete contiene:

- `config/build_config.lua`: ambiente, canal, client version, asset version,
  commit y fecha UTC.
- `artifact-environment.json`: marcador breve para CI y soporte.
- `build-metadata.json`: mismos valores, SHA-256 del ejecutable y configuración
  de conexión sin credenciales.

El auditor exige que un paquete production tenga URL, puerto 443, protocolo
1525 y ambos flags en `false`. También rechaza localhost, RFC1918, endpoints de
test/staging y protocolos alternativos dentro del paquete distribuible.

## Persistencia y migración

Al primer inicio, `ResourceManager::migrateLegacyUserData` importa mediante
allowlist:

- `config.otml`;
- `controls/`, incluidos hotkeys y keybinds normales;
- minimap `.otmm` y `.otcm`;
- screenshots;
- preferencias de outfit y quest tracking;
- listas de texto normales en la raíz.

La migración no sobrescribe archivos existentes y no se ejecuta con
`--user-dir`. Rechaza rutas asociadas con game_bot, vBot, CaveBot, TargetBot,
BotServer, autoheal, autoloot, looter y macros de automatización. Después,
`config/settings_migration.lua` conserva cuenta/password cifrados del endpoint
oficial cuando existen y descarta servidores anteriores.

## Cambio futuro del endpoint

1. Modificar sólo la entrada `production` de `config/environments.lua`.
2. Actualizar el contrato del manifest y sus tests si cambia puerto, protocolo
   o autenticación.
3. Ejecutar `scripts/test-production-config.py` y el auditor sobre ambos
   paquetes finales.
4. Validar certificado, character list e ingreso al game server con una cuenta
   de prueba no personal.
5. Versionar la evidencia y publicar primero un prerelease.
6. Cambiar el manifest stable sólo después de aprobación expresa.

## Pruebas

- `tests/test_thappy_official_client.py` valida la fuente, el bloqueo del login,
  la migración y el perfil retro.
- `tests/evals/eval_official_client.py` exige score 1.00.
- `scripts/test-production-config.py` valida el archivo fuente o un paquete.
- `scripts/audit-client-package.py` valida el contenido final, no los tests del
  checkout.
