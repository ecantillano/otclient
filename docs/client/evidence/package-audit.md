# Evidencia de auditoría de paquetes

Fecha: 2026-07-21

Fuente final: [draft release workflow 29865233634](https://github.com/ecantillano/otclient/actions/runs/29865233634), tag `client-v0.1.0`, commit `75252cf3d5f97a29c35c29ee8901d801bda3e0f5`.

La CI previa al tag también quedó verde en el
[run 29858920649](https://github.com/ecantillano/otclient/actions/runs/29858920649).

## Resultado

| Plataforma | Archivo completo | Comprimido | Instalado | Archivos | SHA-256 | Auditoría |
|---|---|---:|---:|---:|---|---|
| Windows x86_64 | `Thappy-Windows-x86_64-0.1.0.zip` | 29,953,223 B (28.57 MiB) | 51,170,140 B (48.80 MiB) | 1,017 | `94191c8f4ceed259640d8c326416513dda46859e7b5d5a604c06a49ca248f651` | OK, 0 errores, 0 warnings |
| Linux x86_64 | `Thappy-Linux-x86_64-0.1.0.tar.zst` | 28,563,633 B (27.24 MiB) | 54,417,955 B (51.90 MiB) | 1,017 | `f9e24043bef6d9284d4c54b98cddd5f04256438bc944f618106d339ff63edf92` | OK, 0 errores, 0 warnings |

Ambos pasan el límite de 104,857,600 bytes. El artefacto histórico medido era
171,072,354 B comprimido y 763,987,388 B extraído. La reducción es 82.49%/93.30%
en Windows y 83.30%/92.88% en Linux, respectivamente.

## Binarios y componentes

| Binario | Tamaño | SHA-256 | Formato verificado |
|---|---:|---|---|
| `Thappy.exe` | 19,725,824 B | `3efb63a7759a7d969a723013ae7c7482832422771a7c0e2a0af9deb80f306355` | PE32+ GUI x86-64, MSVC 19.51 |
| `ThappyLauncher.exe` | 6,327,296 B | `2e5d66363b057f91e998c213f4e0423bef2431301a579fd228335b6053bef4af` | PE32+ console x86-64 |
| `thappy` | 23,169,912 B | `8cba8024dacaf517e1abc118908c9c11c25b30a001dd2c51b172d95232c8b834` | ELF x86-64 Release, stripped por CI |
| `thappy-launcher` | 6,135,960 B | `be85cbb44a5ea3043c660e99e8335a84a6771a3b48646344eed782fe6ee4ac17` | ELF x86-64, Go `-trimpath -s -w` |

Los manifests dividen cada plataforma en `core`, `modules` y `data`. El
bootstrap queda fuera de OTA para que el launcher no se reemplace mientras se
ejecuta. No se generó componente `assets` porque no se entregaron catálogos 1525
autorizados.

## Gates inspeccionados dentro de los archivos finales

- no existen PDB, ILK, OBJ, LIB, fuentes, headers, build trees, vcpkg ni caches;
- no existe `mods/`, `game_bot`, vBot, CaveBot, TargetBot ni perfiles del BOT;
- no se incluyen módulos visuales modernos prohibidos;
- no existen endpoints privados, localhost, test o staging en production;
- están `LICENSE`, `THIRD_PARTY_NOTICES.md` y `licenses/OTClient-MIT.txt`;
- `SHA256SUMS-<platform>.txt`, SBOM SPDX, manifest y reporte de tamaño fueron
  generados por CI;
- ambos `SHA256SUMS-<platform>.txt` contienen sólo LF y se verificaron desde
  macOS con `shasum -a 256 -c`;
- el ZIP Windows se extrajo en una carpeta vacía y `file` confirmó ambos PE64;
- el job Windows registró `cl.exe` y C/C++ `MSVC 19.51.36248.0`.

La auditoría verifica contenido y layout portable. No afirma boot gráfico ni
gameplay; esas pruebas están bloqueadas por los assets descritos en
`compatibility-test.md`.

## Draft descargado desde GitHub

La publicación contiene 22 adjuntos: paquetes completos, bootstrap, componentes,
manifests por plataforma y unido, auditorías, SBOMs, reportes y checksums. Se
descargaron nuevamente con `gh release download`; los 21 archivos listados en
`SHA256SUMS.txt` verificaron `OK`. El adjunto restante es el propio
`SHA256SUMS.txt`, SHA-256
`53b6790b48586522ba60f4b7df2287ec890650b9fb3bc1b63bad8c1770ae2451`.

El manifest unido contiene seis componentes, `core`/`modules`/`data` para ambas
plataformas, y ninguna entrada `assets`. Conserva production, protocolo 1525,
asset version 1525, update no mandatory y launcher mínimo 0.1.0.
