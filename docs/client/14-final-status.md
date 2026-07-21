# Estado final del candidato oficial

Fecha: 2026-07-21

Estado: **NEEDS_CONTEXT** para validar gameplay y aprobar stable. La ingeniería,
builds, paquetes, launcher, CI y controles de release están completos.

## Candidato

| Campo | Valor |
|---|---|
| Fork | `ecantillano/otclient` |
| Rama | `codex/thappy-official-client` |
| Base upstream | `opentibiabr/otclient@465b7a217` |
| Versión | `0.1.0` |
| Protocolo | `1525` |
| Assets | `1525`, catálogos no suministrados |
| CI validado | run `29852271609`, Windows/Linux/gates/C++ verdes |
| Windows | 28.57 MiB descargado, 48.80 MiB instalado |
| Linux | 27.24 MiB descargado, 51.90 MiB instalado |
| Baseline histórico | 163.15 MiB descargado, 728.60 MiB extraído |
| Publicación | sólo draft/prerelease; stable y manifest estable no autorizados |

## Listo

- identidad Thappy y ejecutables finales;
- production exacto, separado de test y local;
- BOT retirado físicamente y bloqueado por auditor;
- UI moderna fuera del load graph y del paquete, soporte de protocolo conservado;
- migración segura de datos del jugador;
- paquetes full y `bootstrap`/`core`/`modules`/`data` reproducibles;
- launcher transaccional con hashes, allowlists, lock, offline y rollback;
- workflows CI, draft release y manifest protegido;
- SBOM, SHA-256, top 100 y límites de tamaño;
- tests gate y evals para runtime, packaging, updater y workflows;
- draft PR: <https://github.com/ecantillano/otclient/pull/1>.

## Bloqueos para stable

1. Entregar catálogos y recursos 1525 autorizados para
   `data/things/1525/` y `data/sounds/1525/`.
2. Entregar una cuenta de test no personal fuera del repositorio.
3. Ejecutar boot gráfico, character list, ingreso y matriz de gameplay en
   Windows y Linux; guardar capturas reales.
4. Aprobar licencias de los assets y, si se dispone de él, configurar el
   certificado Authenticode y la clave privada de firma del manifest sólo en
   GitHub Secrets.

Hasta completar esos cuatro puntos, la release debe permanecer draft/prerelease,
el manifest stable no se toca y ningún tester recibe una promesa de cliente
jugable.
