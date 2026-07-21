# Proceso de release

## Preparación

1. Actualizar `VERSION` y `ASSET_VERSION`.
2. Actualizar `CHANGELOG.md`.
3. Ejecutar `scripts/test-production-config.py`.
4. Ejecutar tests, evals, builds y auditoría de ambos paquetes.
5. Revisar `THIRD_PARTY_NOTICES.md`, SBOM y permisos de assets.
6. Probar instalación limpia, launcher offline, update, hash inválido y rollback.
7. Probar character list e ingreso al game server con una cuenta de test no guardada.

## Primera draft production

```bash
git tag -s client-v0.1.0 -m "Thappy client 0.1.0"
git push origin client-v0.1.0
```

El push del tag inicia `client-release.yml`. Aunque el canal del artefacto es
`stable` y usa el perfil production, el workflow crea la publicación como
**draft y prerelease**. No publica stable ni actualiza el manifest de jugadores.

Si no existe una clave Git de firma, usar un tag anotado sin `-s` y registrar
esa limitación. Nunca sobrescribir un tag.

## Siguiente prerelease test

1. Cambiar `VERSION` a una versión `X.Y.Z-rc.N`.
2. Configurar las variables de repositorio `THAPPY_TEST_LOGIN_URL`,
   `THAPPY_TEST_LOGIN_PORT`, `THAPPY_TEST_WEBSITE_URL`,
   `THAPPY_TEST_SUPPORT_URL` y `THAPPY_TEST_MANIFEST_URL`.
3. Ejecutar tests, evals y CI.
4. Crear un tag nuevo que coincida exactamente con `VERSION`:

```bash
git tag -s client-v0.1.1-rc.1 -m "Thappy client 0.1.1-rc.1"
git push origin client-v0.1.1-rc.1
```

Como alternativa, una vez que el workflow exista en la rama por defecto, se
puede iniciar manualmente sin crear el tag por adelantado:

```bash
gh workflow run client-release.yml \
  --ref main \
  -f version=0.1.1-rc.1 \
  -f asset_version=1525 \
  -f channel=test \
  -f test_login_url=https://test.example.invalid/login.php \
  -f test_login_port=443 \
  -f test_website_url=https://test.example.invalid \
  -f test_support_url=https://test.example.invalid/support \
  -f test_manifest_url=https://test.example.invalid/manifest-test.json \
  -f mandatory=false
```

Los valores `.invalid` son marcadores y el workflow fallará hasta recibir
endpoints test reales. El dispatch rechaza tags existentes y no sobrescribe
releases.

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
