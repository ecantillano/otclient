# Auditoría de eliminación del BOT

## Resultado

El BOT integrado fue retirado del código fuente, del grafo de carga, de los
recursos y del contrato de paquetes. La carpeta `mods/` tampoco se monta en el
cliente oficial ni se distribuye.

## Inventario y acción

| Ruta o referencia | Función encontrada | Acción | Riesgo controlado | Prueba |
|---|---|---|---|---|
| `mods/game_bot/` | Loader, UI y ejecución del BOT | carpeta completa eliminada | referencias Lua faltantes | scan de rutas y boot smoke test |
| `default_configs/cavebot_1.3/` | CaveBot y TargetBot | eliminado | ninguna función normal depende de estos perfiles | auditor de paquete |
| `default_configs/vBot_4.8/` | vBot, heal/attack, looting, BotServer | eliminado | callbacks externos residuales | auditor de términos y endpoints |
| `mods/client_profiles/` | perfiles de vBot/actionbar y callback `game_bot.refresh` | eliminado | no confundir con hotkeys normales | tests de migración |
| `mods/client_mods/mods.otmod` | carga forzada de `game_bot` | referencias retiradas | módulo fantasma | test de loader |
| `modules/game_interface/gameinterface.lua` | menú de ID para BOT y comentario de pausa | retirado | menú de juego normal | test estático y smoke test |
| `modules/game_actionbar/logics/ActionButtonLogic.lua` | callbacks para macros del BOT | retirado | actionbar moderna ya no se carga | scan semántico |
| `modules/game_npctrade/game_npctrader.lua` | API de venta invocada por vBot | retirada | NPC trade normal se fuerza a modo clásico | test de perfil retro |
| `modules/gamelib/player.lua` | comentarios/marcadores vBot | retirados | enums de estado se conservan | diff de parsers/enums |
| `src/framework/ui/uiwidget.cpp` | helper expuesto para scripts del BOT | retirado | UI normal sigue compilada | build C++ |
| `data/images/options/bot.png` y `data/images/topbuttons/bot.png` | botones del BOT | eliminados/excluidos | referencias faltantes | auditor de paquete |
| estado persistido heredado | config, perfiles y macros | no se importa y se purgan claves exactas | preservar hotkeys normales | tests de migración |

El inventario inicial midió 174 archivos y aproximadamente 1.5 MiB bajo
`mods/game_bot/`. Incluía Lua ejecutable, OTUI, PNG, OGG, CFG y JSON.

## Lo que se conserva

No se eliminan las hotkeys normales para spells, items, runas, mensajes,
interfaz, battle list o target manual. Tampoco se eliminan parsers, opcodes,
features o enums C++ del protocolo 1525.

La migración puede mencionar nombres históricos del BOT sólo dentro de una
allowlist de purga no ejecutable. El auditor reconoce exclusivamente
`config/settings_migration.lua` como esa excepción y rechaza llamadas de juego,
red, carga dinámica o scheduling dentro de ella.

## Gates

- falla si existe `mods/game_bot` o `mods/client_profiles`;
- falla si un paquete contiene `mods/`;
- falla ante rutas o contenido game_bot, vBot, rvBot, CaveBot, TargetBot,
  BotServer, default_configs, autoheal o autoloot;
- falla si reaparece un loader o endpoint del BOT;
- valida que el perfil retro no cargue actionbar ni módulos modernos;
- ejecuta un eval independiente con umbral 1.00.

La evidencia final del paquete se registra en
`docs/client/evidence/package-audit.md` después de compilar y auditar los dos
artefactos.
