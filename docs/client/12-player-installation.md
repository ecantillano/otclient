# Instalación para jugadores

## Windows x64

1. Descargar `Thappy-Windows-x64-VERSION.zip` desde GitHub Releases de `ecantillano/otclient`.
2. Comparar SHA-256 con `SHA256SUMS`.
3. Extraer en una carpeta escribible por el usuario.
4. Ejecutar `ThappyLauncher.exe`.
5. La primera prerelease puede mostrar SmartScreen porque aún no tiene Authenticode.

No instalar dentro de `Program Files` si se desea un modo portable sin permisos de administrador.

## Linux x86_64

```bash
sha256sum -c SHA256SUMS --ignore-missing
tar --zstd -xf Thappy-Linux-x86_64-VERSION.tar.zst
cd Thappy
./thappy-launcher
```

## Diagnóstico

```bash
ThappyLauncher.exe --diagnose
./thappy-launcher --diagnose
```

El diagnóstico muestra versiones, canal, ambiente, plataforma, manifest, endpoint público e integridad. No muestra password, token o sesión.

## Datos personales

Las actualizaciones preservan options, audio, gráficos, hotkeys, minimap, screenshots y preferencias. La migración desde OTClient ignora configuraciones de CaveBot, TargetBot, vBot, macros de BOT y servidores antiguos.

## Offline

Si no hay conexión y el update no es obligatorio, el launcher permite iniciar la versión instalada. Un update obligatorio muestra el error y no modifica archivos.
