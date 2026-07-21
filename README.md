# Thappy

Cliente de escritorio oficial de Thappy para Windows x64 y Linux x86_64.
Mantiene compatibilidad interna con Canary y protocolo 1525, con una interfaz
retro compacta y sin BOT integrado.

Este repositorio es un fork de
[OpenTibiaBR OTClient Redemption](https://github.com/opentibiabr/otclient).
Conserva sus atribuciones y licencia MIT. Thappy no presenta marcas ni recursos
de OpenTibiaBR como propios.

## Estado

Versión candidata: `0.1.0`. La primera publicación debe permanecer como draft o
prerelease. No existe una release stable aprobada.

El código no incluye sprites, catálogos ni sonidos 1525. Los datos autorizados
deben instalarse exactamente en:

```text
data/things/1525/
data/sounds/1525/
```

El cliente no descarga assets de terceros. Sin esos archivos falla al inicio
con un mensaje explícito.

## Contrato de producción

```lua
Servers_init = {
    ["https://login.thappy.cl/login.php"] = {
        port = 443,
        protocol = 1525,
        httpLogin = false,
        useAuthenticator = false
    }
}
```

El build production oculta y bloquea host, puerto y protocolo. La configuración
persistida no puede reemplazar este endpoint.

## Componentes

- `src/`, `modules/`, `data/`, `config/`: runtime OTClient de Thappy.
- `launcher/`: launcher Go con SHA-256, update por componentes, transacciones y
  rollback.
- `scripts/`: packaging reproducible, auditoría, clean install y simulaciones
  de update.
- `.github/workflows/client-*.yml`: CI y release draft/prerelease.
- `docs/client/`: inventario, decisiones, build, release, seguridad y evidencia.

## Verificación rápida

```bash
python3 -m unittest tests.test_thappy_official_client
python3 tests/evals/eval_official_client.py
python3 -m unittest discover -s scripts/tests -p 'test_*.py'
python3 scripts/run-client-release-evals.py
(cd launcher && go test ./... && go vet ./...)
```

Builds y paquetes: [docs/client/08-build.md](docs/client/08-build.md).
Proceso de release: [docs/client/09-release-process.md](docs/client/09-release-process.md).

## Licencias

- [LICENSE](LICENSE)
- [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)
- [licenses/OTClient-MIT.txt](licenses/OTClient-MIT.txt)
