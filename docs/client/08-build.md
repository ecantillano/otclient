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
cmake --preset windows-release -DCMAKE_BUILD_TYPE=Release -DTOGGLE_BIN_FOLDER=ON -DOPTIONS_ENABLE_IPO=ON -DOTCLIENT_BUILD_TESTS=OFF
cmake --build --preset windows-release --config Release --target otclient
python scripts\package-client.py --platform windows-x64 --build-dir build\windows-release --environment production --output-dir dist
python scripts\audit-client-package.py dist\Thappy-Windows-x64-0.1.0.zip --environment production
```

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
python3 scripts/package-client.py --platform linux-x86_64 --build-dir build/linux-release --environment production --output-dir dist
python3 scripts/audit-client-package.py dist/Thappy-Linux-x86_64-0.1.0.tar.zst --environment production
```

El workflow ejecuta en runners x86_64. Un Mac ARM puede probar scripts y launcher, pero no sustituye esos builds.

## Tests y evals

```bash
python3 -m unittest discover -s tests -p 'test_*.py'
python3 scripts/test-production-config.py .
python3 scripts/test-package.py
python3 scripts/test-clean-install.py
python3 scripts/test-update.py
python3 scripts/run-client-evals.py
```

Los comandos exactos pueden requerir argumentos de staging; `--help` es la fuente de verdad del script.
