# Build reproducible

## Versiones y perfiles

- Cliente: `VERSION`.
- Assets: `ASSET_VERSION`.
- Production: endpoint único Thappy, canal stable para release aprobada.
- Test: canal test y endpoint provisto al crear el paquete.
- Local: canal local y endpoint de desarrollo explícito.

El perfil production no acepta localhost, RFC1918, staging ni un protocolo distinto de 1525.

## Windows x64

Requisitos:

- Windows runner x64;
- Visual Studio Build Tools y MSVC v145;
- CMake 3.24 o posterior;
- Ninja;
- vcpkg en el baseline del manifest.

Comandos:

```powershell
$env:VCPKG_ROOT = "C:\vcpkg"
./scripts/build-windows-msvc.ps1 -Preset windows-release
go build -C launcher -trimpath -ldflags="-s -w -X main.buildVersion=0.1.0" -o "$PWD\launcher\bin\ThappyLauncher.exe" ./cmd/thappy-launcher
python scripts\package-client.py package --source-root . --binary build\windows-release\bin\Thappy.exe --launcher-binary launcher\bin\ThappyLauncher.exe --runtime-dir build\windows-release\bin --output-dir dist\windows --platform windows --version 0.1.0 --asset-version 1525 --channel stable --environment production --budget 104857600 --base-url https://github.com/ecantillano/otclient/releases/download/client-v0.1.0
python scripts\audit-client-package.py dist\windows\Thappy-Windows-x86_64-0.1.0.zip --environment production
```

`build-windows-msvc.ps1` entra al entorno de Visual Studio, exige `cl.exe` y
ejecuta configure y build en el mismo proceso. El build falla cerrado si MSVC
no está disponible; no acepta MinGW encontrado accidentalmente en `PATH`.

El paquete público no incluye PDB, ILK, LIB, OBJ, vcpkg, source tree ni cache. Los símbolos pueden guardarse como artifact privado con retención limitada.

## Linux x86_64

Requisitos:

- Ubuntu x86_64;
- CMake 3.24 o posterior;
- Ninja;
- vcpkg;
- `zstd` para el archivo final.

Comandos:

```bash
export VCPKG_ROOT=/opt/vcpkg
cmake --preset linux-release -DCMAKE_BUILD_TYPE=Release -DTOGGLE_BIN_FOLDER=ON -DOPTIONS_ENABLE_IPO=ON -DOTCLIENT_BUILD_TESTS=OFF
cmake --build --preset linux-release --target otclient
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -C launcher -trimpath -ldflags="-s -w -X main.buildVersion=0.1.0" -o "$PWD/launcher/bin/thappy-launcher" ./cmd/thappy-launcher
python3 scripts/package-client.py package --source-root . --binary build/linux-release/bin/thappy --launcher-binary launcher/bin/thappy-launcher --runtime-dir build/linux-release/bin --output-dir dist/linux --platform linux --version 0.1.0 --asset-version 1525 --channel stable --environment production --budget 104857600 --base-url https://github.com/ecantillano/otclient/releases/download/client-v0.1.0
python3 scripts/audit-client-package.py dist/linux/Thappy-Linux-x86_64-0.1.0.tar.zst --environment production
```

El workflow ejecuta en runners x86_64. Un Mac ARM puede probar scripts y launcher, pero no sustituye esos builds.

## Tests y evals

```bash
python3 -m unittest tests.test_thappy_official_client
python3 tests/evals/eval_official_client.py
python3 -m unittest discover -s scripts/tests -p 'test_*.py'
python3 scripts/run-client-release-evals.py
(cd launcher && go test ./... && go test -race ./... && go vet ./... && go test -tags=eval ./evals)
```
