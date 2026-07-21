# Evidencia de compatibilidad Thappy

Fecha: 2026-07-21

## Verificado

- build C++ y tests C++ pasan en Ubuntu x86_64;
- build production Windows x86_64 pasa con MSVC 19.51;
- build production Linux x86_64 pasa en Ubuntu 24.04 y se publica stripped;
- los dos paquetes se extraen fuera del build tree y contienen ejecutable,
  launcher, `data/`, `modules/` y licencias;
- el grafo retro conserva login, character list, inventory, containers, battle,
  console, minimap, skills, VIP, hotkeys, trade, NPC trade, party, guild, outfit,
  quest log, condiciones y cooldowns;
- parsers, opcodes, enums y feature flags del protocolo moderno permanecen en
  C++, aunque las UIs modernas no se cargan ni se empaquetan;
- producción queda fijada al protocolo 1525 y al endpoint canónico;
- la ausencia de catálogos se convierte en error fatal previo a cargar módulos,
  no en descarga silenciosa desde terceros.

## No verificado y motivo

No se pudo abrir la pantalla gráfica, obtener character list, ingresar al game
server ni ejecutar caminar, cambio de piso, combate, experiencia, container,
item, consola y relog. Faltan estos insumos indispensables:

```text
data/things/1525/catalog-content.json
data/sounds/1525/catalog-sound.json
cuenta de test no personal
```

Los catálogos y recursos asociados deben tener autorización de distribución.
No se buscaron ni incluyeron copias de terceros. Por la misma razón no existen
`login.png`, `character-list.png`, `ingame.png` ni `modules.png`: crear esas
capturas sin arrancar el paquete final sería evidencia falsa.

## Dictamen

La compatibilidad estructural, de compilación, perfil, protocolo y paquete está
verde. La compatibilidad funcional contra Thappy sigue pendiente y bloquea la
publicación stable.
