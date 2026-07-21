# Thappy Launcher

Launcher portable y separado de Thappy, escrito sólo con la biblioteca estándar
de Go. Consulta un manifest estático por HTTPS, descarga únicamente componentes
con una versión distinta, verifica SHA-256 antes de extraer y conserva una
transacción reversible hasta validar el primer arranque controlado del cliente.

## Resultado observable

- `stable` y `test` usan manifests distintos y `stable` rechaza prereleases.
- Production sólo acepta `https://login.thappy.cl/login.php`, puerto `443`,
  protocolo `1525`, `http_login=false` y `use_authenticator=false`.
- `asset_version` acepta el identificador canónico `1525`; no se interpreta como
  SemVer. Las versiones del launcher, cliente y componentes sí son SemVer 2.0.
- La consola muestra el total de bytes de los componentes cambiados y el avance
  por componente. Un componente con la misma versión no se descarga.
- Si la red falla, arranca la última instalación confirmada salvo que el último
  manifest válido marque una actualización pendiente como `mandatory`.
- Settings, hotkeys, minimap, screenshots, logs, perfiles y preferencias están
  fuera de los targets actualizables y de la allowlist de borrado.

## Compilar y verificar

Requiere Go 1.23 o posterior. Los builds no usan CGO ni incluyen fuentes,
símbolos o caches:

```sh
make test
make eval
make build
```

Salidas:

- `dist/ThappyLauncher.exe`: Windows x64.
- `dist/thappy-launcher`: Linux x86_64.

La lane rápida es `go test ./...`; la lane de contrato es
`go test -tags=eval ./evals`. Esta última lee `evals/cases.json` y valida SemVer,
rutas hostiles y variantes de manifest sin usar red.

## Configuración

Copiar `launcher-config.example.json` como `launcher-config.json` junto al
launcher y reemplazar las URLs cuando existan los assets reales del prerelease.
El archivo local es la raíz de confianza para hosts, archivos preservados y
borrados permitidos. Un manifest remoto no puede ampliar esas listas.

Los manifests de referencia son `manifest-stable.example.json` y
`manifest-test.example.json`. El schema efectivo usa snake_case:

```json
{
  "schema_version": 1,
  "channel": "stable",
  "version": "0.1.1",
  "protocol_version": 1525,
  "asset_version": "1525",
  "release_notes_url": "https://github.com/ecantillano/otclient/releases/tag/v0.1.1",
  "mandatory": false,
  "environment": "production",
  "login_url": "https://login.thappy.cl/login.php",
  "login_port": 443,
  "http_login": false,
  "use_authenticator": false,
  "minimum_launcher_version": "0.1.0",
  "components": [],
  "delete": []
}
```

Cada componente exige `name`, SemVer en `version`, `url` HTTPS, SHA-256 de 64
caracteres, tamaño exacto, `archive: "zip"`, plataformas opcionales y un target
relativo. El manifest completo tiene límite de 1 MiB. Los ZIP tienen límites de
archivos y tamaño descomprimido, rechazan symlinks, rutas absolutas, `..`, ADS,
nombres reservados de Windows y colisiones por mayúsculas/minúsculas.

## Flujo transaccional

1. Recupera primero un journal interrumpido.
2. Valida manifest, canal, endpoint, hosts, SemVer y versión mínima del launcher.
3. Guarda la política `mandatory` válida localmente antes de descargar.
4. Calcula sólo componentes con versión distinta y comprueba espacio disponible.
5. Descarga con timeout, hasta tres intentos y redirects validados uno por uno.
6. Verifica tamaño y SHA-256 mientras escribe en staging.
7. Extrae en staging y aplica renames dentro de la instalación. Los originales
   pasan a `.thappy-launcher/backups/<transaction>`.
8. Guarda el estado como `pending_launch` y ejecuta Thappy manteniendo el lock.
9. Si Thappy termina con código cero, confirma `last_good_version` y elimina el
   backup. Si falla, restaura archivos y estado anteriores.

Un cierre abrupto antes de la confirmación conserva el journal; el siguiente
inicio revierte ese update antes de consultar la red. `--no-launch` deja el
update pendiente y por diseño también se revierte en el próximo inicio si nunca
hubo arranque controlado.

## Uso

```sh
./thappy-launcher
./thappy-launcher --channel test
./thappy-launcher --no-launch
./thappy-launcher --diagnose
```

Flags adicionales: `--install-dir`, `--config`, `--manifest-url`, `--client` y
`--allow-host`. Los overrides de URL siguen sujetos a HTTPS y allowlist exacta.

`--diagnose` imprime JSON redactado con versiones, canal, ambiente, plataforma,
arquitectura, manifest, endpoint canónico, puerto, protocolo, flags de login,
componentes, hashes de paquetes verificados, edad del estado, último error,
integridad, lock, espacio disponible y ruta del log. Queries, userinfo y rutas
debajo del home se redactan.

Los eventos persistentes viven en `.thappy-launcher/logs/launcher.log` como
JSONL con permisos privados. Rotan a 1 MiB, conservando dos archivos anteriores.
No se registran manifests completos, argumentos del cliente, sesiones ni claves.

## Límites deliberados de la primera versión

- El launcher no se autoactualiza. Si `minimum_launcher_version` es superior,
  falla cerrado con una instrucción explícita para actualizar Thappy Launcher de
  forma manual. Nunca intenta reemplazar su propio ejecutable en uso.
- No hay una clave Ed25519 disponible. La integridad usa HTTPS + allowlist exacta
  + tamaño + SHA-256. La primera release debe seguir siendo prerelease hasta que
  CI firme el manifest y se agregue verificación con clave pública embebida.
- Un transfer truncado se vuelve a descargar con intentos acotados; no existe
  reanudación por rangos en esta versión.
- El lock detecta otra instancia del launcher y mantiene bloqueado el update
  mientras el cliente lanzado por él está activo. Un cliente abierto directamente
  fuera del launcher sólo se detecta cuando el sistema operativo rechaza el
  reemplazo; esa falla dispara rollback.
- La confirmación interpreta salida cero del proceso gestionado como ejecución
  correcta. Un crash o salida distinta de cero revierte el update.

No requiere administrador si la instalación portable está en una carpeta
escribible por el usuario. No ejecutar el launcher desde `Program Files` salvo que
la política de permisos de la instalación lo permita.
