# Estrategia de repositorios

## Decisión

El cliente se desarrolla en `ecantillano/otclient`, branch `codex/thappy-official-client`, creada desde `origin/main` en `465b7a217e87502bb7f9980bf6e099718d0a9a49`.

No se usa `thappy-windows` como base porque no tiene commits propios y está 16 commits detrás. Se conserva sin reescritura para auditoría histórica.

El launcher se implementa inicialmente en `launcher/` con límite de servicio propio: código, tests, manifest schema y README no dependen de internals C++ de OTClient. Esto permite validarlo en el mismo PR y extraerlo a `ecantillano/thappy-launcher` sin reescribir su diseño. El repositorio separado no existía al iniciar el trabajo. No se crea público hasta revisar licencias, secretos y artefactos.

## Sincronización con upstream

Remotes locales:

```text
origin   https://github.com/ecantillano/otclient.git
upstream https://github.com/opentibiabr/otclient.git
```

Flujo:

```bash
git fetch upstream main
git switch codex/thappy-official-client
git merge --no-ff upstream/main
```

La personalización se concentra en configuración, loader, packaging, launcher, docs y workflows. No se eliminan enums, opcodes ni parsers de protocolo. Esto reduce conflictos cuando upstream cambia C++.

## Contratos entre cliente y launcher

- El launcher consume un manifest versionado, nunca internals del cliente.
- Los paquetes escriben sólo dentro del directorio de instalación.
- El runtime mantiene `data/things/<version>/` y `data/sounds/<version>/` como rutas canónicas.
- Los datos del jugador viven fuera de los componentes actualizables.
- El cliente expone su ambiente y versión mediante archivos generados en el paquete.

## Política de ramas, tags y releases

- No force push.
- No sobrescribir tags.
- Tags de prueba: `client-vX.Y.Z-rc.N`.
- Tags estables: `client-vX.Y.Z`, sólo después de aprobación.
- La primera release será draft o prerelease.
- `manifest-stable.json` no se publica ni modifica para jugadores reales durante esta etapa.

## Visibilidad y licencias

El fork actual es público y conserva la licencia MIT de OTClient. No se incluirán sprites, catálogos o sonidos 1525 hasta confirmar autorización. Los paquetes de código deben llevar `LICENSE`, `THIRD_PARTY_NOTICES.md` y `licenses/OTClient-MIT.txt`.
