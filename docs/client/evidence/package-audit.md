# Evidencia de auditoría de paquetes

Fecha: 2026-07-21

Fuente: [Thappy Client CI run 29858920649](https://github.com/ecantillano/otclient/actions/runs/29858920649), branch head `989b328ac70d037e7491d84987a42a4bc66ebc70`.

## Resultado

| Plataforma | Archivo completo | Comprimido | Instalado | Archivos | SHA-256 | Auditoría |
|---|---|---:|---:|---:|---|---|
| Windows x86_64 | `Thappy-Windows-x86_64-0.1.0.zip` | 29,953,212 B (28.57 MiB) | 51,170,140 B (48.80 MiB) | 1,017 | `8a9a752930eb0944854a362c4ae19a144a509d4144f0f4e44bfad1b49e9cdd0b` | OK, 0 errores, 0 warnings |
| Linux x86_64 | `Thappy-Linux-x86_64-0.1.0.tar.zst` | 28,563,971 B (27.24 MiB) | 54,417,955 B (51.90 MiB) | 1,017 | `e85e0916bce56692695e03a5744e72b5b27889cfd468256d3b828a3f6b856a0b` | OK, 0 errores, 0 warnings |

Ambos pasan el límite de 104,857,600 bytes. El artefacto histórico medido era
171,072,354 B comprimido y 763,987,388 B extraído. La reducción es 82.49%/93.30%
en Windows y 83.30%/92.88% en Linux, respectivamente.

## Binarios y componentes

| Binario | Tamaño | SHA-256 | Formato verificado |
|---|---:|---|---|
| `Thappy.exe` | 19,725,824 B | `906a912b1874a5692e9ed899d18254677c8f91a94d1a47fdcab8c55fa8fef91a` | PE32+ GUI x86-64, MSVC 19.51 |
| `ThappyLauncher.exe` | 6,327,296 B | `d443fbda78a4c7994dcf015fe6c720b13f01108866a9947a80c55ade75b0eaee` | PE32+ console x86-64 |
| `thappy` | 23,169,912 B | `0928f64ed674cf16d801987653651cf801426c4a7a48e7e5efa59be89b71ad15` | ELF x86-64 Release, stripped por CI |
| `thappy-launcher` | 6,135,960 B | `b951af45e2320523672f1201f718104eda1b8248aed7f8c07f209f01a67e2dd9` | ELF x86-64, Go `-trimpath -s -w` |

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
