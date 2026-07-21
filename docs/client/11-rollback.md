# Rollback

## En el equipo del jugador

El launcher mantiene una copia mínima de los archivos reemplazados y el estado anterior. Una actualización no cambia la versión local hasta completar descarga, hash, extracción y commit.

Ante error:

1. detener el commit;
2. retirar la instalación candidata;
3. restaurar el backup por rename;
4. restaurar el estado local anterior;
5. conservar settings, hotkeys y minimap;
6. registrar fase y error sin credenciales;
7. permitir iniciar la versión anterior si el update no era obligatorio.

Staging y backup deben estar bajo un directorio controlado por el launcher. No se sigue symlinks fuera de la instalación.

## En publicación

1. Identificar último manifest sano.
2. Verificar que sus artifacts todavía existen y que `SHA256SUMS` coincide.
3. Restaurar el manifest del canal afectado.
4. No borrar la release defectuosa; marcarla y explicar el problema.
5. Probar una instalación real contra el manifest restaurado.
6. Preparar una nueva prerelease con versión superior.

`manifest-stable.json` sólo se cambia con aprobación. El canal test puede retroceder a una prerelease anterior sin tocar stable.
