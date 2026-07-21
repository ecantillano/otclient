# Estado actual del cliente

Fecha del inventario: 2026-07-21

Repositorio inspeccionado: `ecantillano/otclient`

Fork de: `opentibiabr/otclient`

## Estado Git

| Dato | Resultado |
|---|---|
| Rama por defecto | `main` |
| Commit de `origin/main` | `465b7a217e87502bb7f9980bf6e099718d0a9a49` |
| Commit de `upstream/main` | `465b7a217e87502bb7f9980bf6e099718d0a9a49` |
| Rama histórica | `thappy-windows` en `bdea0b23b4a738809d698cb7e4f88a299dd6bffc` |
| Relación histórica | 0 commits propios, 16 commits detrás de `main` |
| Tags del fork | ninguno |
| Rama de trabajo | `codex/thappy-official-client` desde `origin/main` |
| Repositorio de launcher | no existía al iniciar el trabajo |

`thappy-windows` no contiene una personalización de Thappy. Es una copia antigua de upstream. Se conserva sin cambios.

## Configuración observada antes de los cambios

`init.lua` contenía dos ejemplos upstream:

- `http://127.0.0.1/login.php`, puerto 80, protocolo 1511, `httpLogin=true`.
- `ip.net`, puerto 7171, protocolo 860, `httpLogin=false`.

La configuración production de Thappy no estaba versionada. El cliente anterior pudo conectarse sólo mediante edición manual o estado persistido. El build no podía garantizar el endpoint usado por el jugador.

El cliente soporta 1525. `getClientProtocolVersion(1525)` devuelve 1525. Los assets esperados por el runtime para esa versión viven en `data/things/1525/` y `data/sounds/1525/`.

## Build

- CMake raíz 3.16; presets requieren CMake 3.24.
- MSVC usa C++20; las demás plataformas usan C++23.
- Windows Release existente: x64, `RelWithDebInfo`, vcpkg estático, toolset v145.
- Linux tiene presets Release y Debug con Ninja.
- Baseline vcpkg: `cd61e1e26a038e82d6550a3ebbe0fbbfe7da78e3`.
- `OTCLIENT_BUILD_TESTS` existe y está desactivado en Release.
- El build desktop sólo producía el binario. No existía una etapa de instalación o paquete portable.

El único run del fork era Actions `29675622968`, branch `thappy-windows`, commit `bdea0b23...`. Sus 11 jobs terminaron con éxito. No existía un run del fork para `main`.

## Estructura runtime

| Directorio | Tamaño del checkout | Función |
|---|---:|---|
| `modules/` | 20 MiB | Lua, OTUI y módulos del cliente |
| `mods/` | 1.6 MiB | extensiones cargadas después de los módulos base |
| `data/` | 31 MiB | imágenes, fuentes, estilos, locales, shaders y placeholders de assets |
| `src/` | 4.6 MiB | código C++ que no debe entrar al paquete público |

PhysFS monta el directorio de trabajo, después `data`, `modules`, `mods` y paquetes `.otpkg`. Los `.otmod` se descubren y cargan por prioridad. `client`, `game_interface` y `client_mods` fuerzan grandes listas `load-later`.

## Persistencia observada

Antes del cambio, la identidad era `OTClient - Redemption`, compact name `otclient` y organización `otcr`. PhysFS calculaba el directorio de preferencias con esos nombres y luego agregaba `otclient/` en Windows o `.otclient/` fuera de Windows.

Datos persistidos:

- `config.otml`: opciones, host, puerto, versión, ServerList y credenciales recordadas cifradas por el cliente.
- `controls/keybinds/<preset>.otml` y `controls/hotkeys/<preset>.otml`.
- `minimap.otmm` o `minimap_<version>.otcm`.
- capturas, preferencias de ventana y estado de módulos.
- BOT: `bot/<config>/storage/profile_<profile>.json` y `config.otml.bot`.

El argumento `--user-dir=<path>` de `main` permite pruebas aisladas. Los logs normales se escribían en el directorio runtime como `otclient.log`; los crash reports también dependían del directorio de trabajo.

## BOT

El BOT no estaba oculto: se cargaba siempre desde `mods/client_mods/mods.otmod`. `mods/game_bot/` contenía 174 archivos y 1.5 MiB con CaveBot, TargetBot, vBot, macros, heal/attack, looting, perfiles, sonidos, UI, BotServer y endpoints externos.

También había referencias fuera de esa carpeta en:

- `mods/client_profiles/profiles.lua`
- `modules/game_interface/gameinterface.lua`
- `modules/game_actionbar/logics/ActionButtonLogic.lua`
- `modules/game_npctrade/game_npctrader.lua`
- `modules/gamelib/player.lua`
- `src/framework/ui/uiwidget.cpp`
- `data/images/options/bot.png`
- `data/images/topbuttons/bot.png`

La auditoría y el resultado de la eliminación se documentan en `04-bot-removal-audit.md`.

## Sistemas modernos

El protocolo 1525 activa features por número de cliente, aunque el ruleset de Thappy sea retro. La primera etapa debe retirar carga, botones y UI sin borrar parsers, opcodes o enums C++. La matriz completa está en `05-modern-modules-matrix.md`.

## Updater y cifrado heredados

- El updater upstream estaba desactivado.
- Usa CRC32, no una firma ni SHA-256.
- Puede descargar un ejecutable nuevo, pero no ofrece un commit atómico por componentes con rollback probado.
- El cifrado upstream está desactivado y es un esquema reversible no autenticado.
- No se habilitará ese cifrado ni se usará como control de integridad.

## Resultado del diagnóstico

La base adecuada es `main`, no `thappy-windows`. El trabajo requiere crear configuración production inmutable, retirar el BOT real, adelgazar el grafo visual, construir paquetes runtime explícitos y usar un launcher con SHA-256, staging y rollback.
