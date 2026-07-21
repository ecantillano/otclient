# Arquitectura del updater

## Componentes

```text
bootstrap  launcher + configuración mínima
core       Thappy.exe/thappy + DLL/SO runtime
modules    Lua + OTUI del perfil retro
data       configuración, imágenes, fuentes, locales y shaders autorizados
assets     data/things/1525 + data/sounds/1525 cuando exista autorización
```

El launcher consume un manifest estático por HTTPS. GitHub API no es parte del path normal de actualización.

## Flujo

1. Leer versión, canal y estado local.
2. Descargar manifest con timeout y límite de tamaño.
3. Validar HTTPS, hostname permitido, schema y semantic version.
4. Rechazar rutas absolutas, `..`, backslashes peligrosos y targets fuera de instalación.
5. Comparar hashes/versiones de componentes.
6. Calcular descarga y espacio requerido.
7. Descargar a staging dentro del mismo filesystem cuando sea posible.
8. Verificar SHA-256 antes de abrir o extraer.
9. Extraer con defensa Zip Slip.
10. Validar instalación candidata.
11. Mover la instalación previa a backup.
12. Aplicar por rename atómico.
13. Procesar `delete` sólo dentro del root permitido.
14. Escribir estado local al final.
15. Iniciar Thappy y conservar backup hasta confirmar arranque.
16. Revertir en el siguiente inicio si queda un journal sin confirmar.

## Datos excluidos del update

- settings y preferencias de ventana;
- hotkeys y keybinds;
- minimap;
- screenshots;
- logs y crash reports;
- listas locales;
- cache no esencial.

Estos datos viven en el directorio de preferencias de Thappy o en el `--user-dir` indicado, nunca dentro de un componente reemplazable.

## Seguridad

- SHA-256 obligatorio.
- HTTPS obligatorio en ambientes no locales.
- allowlist de hosts.
- redirects validados por cada salto.
- ningún comando shell construido desde el manifest.
- ningún archivo se ejecuta antes de verificar hash.
- límite de reintentos y timeout.
- launcher y cliente directo comparten un lock de instalación;
- manifest firmado preparado, pero no declarado activo sin una clave pública y un test real.

## Actualización del launcher

La versión inicial no reemplaza el launcher en caliente. Los manifests OTA
omiten `bootstrap`; el ZIP bootstrap existe sólo para instalación o reemplazo
manual. `minimum_launcher_version` bloquea con instrucciones explícitas cuando
el launcher instalado es demasiado antiguo.

## Asset contract

Los assets finales siguen en `data/things/<version>/` y `data/sounds/<version>/`. No se crea una fuente runtime alternativa. Los defaults permanecen `strictManifestSha256=true` y `allowRawFallbackHashMismatch=false`.
