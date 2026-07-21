# Proceso de release

## Preparación

1. Actualizar `VERSION` y `ASSET_VERSION`.
2. Actualizar `CHANGELOG.md`.
3. Ejecutar `scripts/test-production-config.py`.
4. Ejecutar tests, evals, builds y auditoría de ambos paquetes.
5. Revisar `THIRD_PARTY_NOTICES.md`, SBOM y permisos de assets.
6. Probar instalación limpia, launcher offline, update, hash inválido y rollback.
7. Probar character list e ingreso al game server con una cuenta de test no guardada.

## Primera prerelease

```bash
git tag -s client-v0.1.0-rc.1 -m "Thappy client 0.1.0-rc.1"
git push origin client-v0.1.0-rc.1
gh workflow run client-release.yml --ref client-v0.1.0-rc.1 -f publish_mode=prerelease
```

Si no existe una clave Git de firma, usar un tag anotado sin `-s` y registrar esa limitación. Nunca sobrescribir un tag.

El workflow debe:

1. Validar tag contra `VERSION`.
2. Hacer checkout limpio.
3. Compilar Windows y Linux Release.
4. Generar paquetes completos y por componente.
5. Ejecutar auditorías.
6. Crear `SHA256SUMS` y manifests.
7. Adjuntar SBOM y reportes.
8. Crear GitHub Release draft o prerelease.

## Aprobación estable

Sólo después de una prerelease aprobada:

1. Descargar artifacts desde GitHub y volver a calcular hashes.
2. Probar instalación limpia en Windows x64 y Linux x86_64.
3. Probar conexión, gameplay y relog.
4. Probar actualización desde la versión anterior.
5. Confirmar que no existe BOT ni UI moderna prohibida.
6. Publicar la release.
7. Actualizar `manifest-stable.json` con aprobación explícita.
8. Verificar un update real.

## Reversión de release

No borrar ni sobrescribir artifacts. Volver el manifest al último conjunto conocido de hashes y versiones. La guía operativa está en `11-rollback.md`.
