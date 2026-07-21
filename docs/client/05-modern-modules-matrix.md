# Matriz de sistemas modernos

Fecha de auditoría: 2026-07-21  
Perfil objetivo: Thappy retro sobre protocolo 1525.

## Regla aplicada

El número de protocolo activa features modernas en `modules/game_features/features.lua`. Eso no prueba que Thappy use su UI o gameplay. En esta etapa se retira el módulo visual y sus accesos. Los parsers, opcodes, enums y eventos C++ se conservan para que un paquete inesperado del servidor no desincronice el stream.

| Sistema | UI/módulo observado | Soporte interno | Clasificación | Acción inicial | Riesgo |
|---|---|---|---|---|---|
| Mounts | `game_playermount`, `game_outfit` | C++ y feature flags | soporte sin promoción visual | ocultar accesos dedicados; conservar outfit compatible | medio |
| Offline training | `game_skills/offlinetraining1524.lua` | eventos de skills | soporte sin UI | no cargar modal moderno | bajo |
| Exercise weapons | sin módulo dedicado | items/protocolo genérico | no presente | ninguna | bajo |
| Prey | `game_prey` | parser/send/event | excluir UI | quitar del load graph | medio |
| Hunting Tasks modernas | `game_taskboard` | Lua/protocolo | excluir UI | quitar autoload/load graph | medio |
| Bestiary / Charms | `game_cyclopedia` | parser/send/event | excluir UI | quitar acceso y módulo | medio |
| Bosstiary | `game_cyclopedia` | parser/send/event | excluir UI | quitar acceso y módulo | medio |
| Imbuements | `game_imbuing`, `game_imbuementtracker` | parser/send/event | excluir UI | quitar módulos | medio |
| Forge | `game_forge` | parser/send/event | excluir UI | quitar módulo | medio |
| Wheel of Destiny | `game_wheel` | parser/send/event | excluir UI | quitar módulo | medio |
| Hazard | sólo strings/achievements | soporte indirecto | no presente como UI | ninguna | bajo |
| Loyalty | campos de character/protocol | parser | soporte sin UI dedicada | conservar | bajo |
| Familiars | `game_outfit` | outfit/protocolo | soporte compartido | no añadir acceso dedicado | medio |
| Quick loot | `game_quickloot`, hooks de interface | parser/send/event | excluir UI | quitar módulo y hooks visuales | medio |
| Auto-loot | BOT | funciones BOT | eliminar | retirar junto al BOT | bajo |
| Auto-bank | sin módulo dedicado | no confirmado | no presente | ninguna | bajo |
| Gold pouch | slot genérico `GamePurseSlot` | protocolo | soporte interno | no mostrar feature comercial | medio |
| Stash | `game_stash` | parser/send/event | excluir UI | quitar módulo | medio |
| Market global | `game_market` | parser/send/event | excluir UI | quitar módulo | medio |
| Reward chest | sin módulo dedicado | contenedores genéricos | no presente | ninguna | bajo |
| Daily rewards / wall | `game_rewardwall`, campos en charlist | parser/send/event | excluir UI | quitar módulo y acceso | medio |
| Loot boosts | sin módulo dedicado | no confirmado | no presente | ninguna | bajo |
| Store / store inbox | `game_shop`, `game_store` | parser/send/event | excluir UI | quitar módulos y botones | medio |
| Cyclopedia moderna | `game_cyclopedia` | parser/send/event | excluir UI | quitar módulo | medio |
| House auction | constantes/send/parse | C++/Lua protocol | soporte sin UI dedicada | conservar internals | medio |
| Task/reward board | `game_taskboard`, `game_rewardwall` | parser/send/event | excluir UI | quitar módulos | medio |
| Analyzers | `game_analyser`, `game_lootsplitter` | Lua | excluir UI | quitar autoload/load graph | bajo |
| Action bars modernas | `game_actionbar` | hotkeys compartidas | retirar UI, conservar hotkeys clásicas | limpiar dependencias antes de descargar | alto |
| Bottom menu moderno | `client_bottommenu` | UI solamente | excluir UI | quitar carga y llamadas guardadas | bajo |
| HUD moderno | `game_healthcircle`, paperdolls/stats bar | eventos compartidos | excluir UI | quitar módulos visuales; conservar health/mana tradicionales | medio |
| Teleports / instancias | sin módulo dedicado | protocolo genérico | no presente | ninguna | bajo |
| Promotions/widgets comerciales | bottom menu/store | UI | excluir UI | quitar carga y recursos | bajo |

## Módulos visuales retirables

Primera lista de exclusión del perfil retro:

```text
client_bottommenu
game_actionbar
game_analyser
game_cyclopedia
game_forge
game_healthcircle
game_imbuementtracker
game_imbuing
game_lootsplitter
game_market
game_prey
game_proficiency
game_quickloot
game_rewardwall
game_shop
game_stash
game_store
game_taskboard
game_tutorial
game_wheel
```

El código puede quedar en el checkout para facilitar merges upstream, pero no debe cargarse ni entrar al paquete `modules` production cuando el auditor lo marca como módulo prohibido. La exclusión física del paquete se decide después de pasar el smoke de Lua.

## Funciones retro que deben seguir cargadas

- login y character list;
- interface base y mapa;
- inventory y equipment;
- containers;
- battle list;
- console y canales;
- skills clásicos;
- VIP;
- minimap;
- hotkeys tradicionales;
- NPC trade y player trade;
- party y shared experience;
- guilds;
- outfit compatible;
- quest log simple;
- condiciones y cooldowns requeridos por protocolo;
- options, logout y reportes de error.

## Pruebas exigidas

1. Resolver el grafo `.otmod` sin dependencias faltantes.
2. Buscar módulos prohibidos en el staging production.
3. Confirmar por código que parsers/opcodes C++ permanecen.
4. Iniciar el cliente con user dir vacío y revisar errores Lua.
5. Probar login y gameplay con una cuenta de test fuera de CI.

Los puntos 4 y 5 requieren un runner gráfico x86_64 y assets 1525 autorizados. Si faltan, se registran como bloqueo explícito, no como éxito inferido.
