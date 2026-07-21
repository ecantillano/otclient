# Evidencia de actualización

Fecha: 2026-07-21

## Resultado

Las pruebas deterministas del launcher pasaron para:

- actualización `0.1.0` a `0.1.1` descargando sólo el componente cambiado;
- omisión del componente sin cambios;
- preservación de settings, hotkeys, minimap, screenshots y logs;
- borrado sólo dentro de la allowlist exacta;
- confirmación de `last-good` únicamente después de un inicio correcto;
- interrupción durante apply y recuperación de la versión `0.1.0`;
- hash SHA-256 inválido rechazado antes de modificar la instalación;
- manifest, URL, redirect, path traversal, Zip Slip, symlink y tamaño inválidos
  rechazados;
- fallo de inicio posterior al update con rollback a `0.1.0`;
- limpieza de journal y staging al confirmar;
- launcher en ejecución preservado y actualización del launcher dejada manual.

Comandos:

```bash
cd launcher
go test ./...
go test -race ./...
go vet ./...
go test -tags=eval ./evals
```

La integración de empaquetado también aplicó los tres componentes reales de
cada manifest CI en una instalación temporal y conservó
`ThappyLauncher.exe`/`thappy-launcher`, `config.otml` y `logs/client.log`.

## Offline

La suite cubre dos políticas: una instalación verificada arranca offline cuando
el último manifest conocido no marca update obligatorio; una versión mandatory
pendiente bloquea el arranque offline. Un journal sucio también bloquea el
arranque hasta recuperar o revertir, evitando usar una instalación parcial.

## Límite de esta evidencia

El flujo ejecuta un proceso de cliente simulado para probar confirmación y fallo
de inicio. No se lanzó el binario Thappy final porque faltan los catálogos 1525
autorizados; por diseño, el runtime se niega a iniciar sin ellos.
