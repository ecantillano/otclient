# Firma de código

## Estado de la primera prerelease

No se encontró un certificado Authenticode ni un secreto de firma. El artifact anterior tiene Security Directory `0 0`, por lo que no está firmado.

La primera prerelease debe declarar de forma visible:

- binarios sin Authenticode;
- hashes SHA-256 publicados en `SHA256SUMS`;
- descarga sólo desde GitHub Releases del fork oficial;
- posible advertencia de SmartScreen por falta de reputación;
- prohibición de desactivar SmartScreen o TLS.

No se usa firma simulada, certificado autofirmado ni UPX.

## Integración futura

Secrets sugeridos:

```text
WINDOWS_SIGNING_CERT_PFX_BASE64
WINDOWS_SIGNING_CERT_PASSWORD
WINDOWS_SIGNING_TIMESTAMP_URL
```

Flujo futuro:

1. Decodificar el PFX dentro del runner a un archivo temporal.
2. Importarlo sólo al store temporal del job.
3. Firmar `Thappy.exe` y `ThappyLauncher.exe` con SHA-256 y timestamp RFC 3161.
4. Verificar con `signtool verify /pa /all /v`.
5. Borrar el archivo temporal y el store.
6. Ejecutar packaging y hashes después de firmar.
7. No imprimir password, certificado, thumbprint privado ni contenido base64.

Ejemplo conceptual del comando en runner Windows:

```powershell
signtool sign /fd SHA256 /tr $env:WINDOWS_SIGNING_TIMESTAMP_URL /td SHA256 /f signing.pfx /p $env:WINDOWS_SIGNING_CERT_PASSWORD Thappy.exe
signtool verify /pa /all /v Thappy.exe
```

La release debe fallar si existe configuración de firma pero la verificación no pasa.

## Rotación

1. Añadir el certificado nuevo como secrets con sufijo de versión.
2. Firmar una prerelease y verificar la cadena en una VM limpia.
3. Cambiar los secrets activos.
4. Revocar el certificado anterior si corresponde.
5. Documentar fecha, issuer, vigencia y motivo sin registrar claves privadas.
