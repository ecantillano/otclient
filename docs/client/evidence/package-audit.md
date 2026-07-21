# Evidencia de auditoría de paquetes

Fecha: 2026-07-21

Fuente: [Thappy Client CI run 29852271609](https://github.com/ecantillano/otclient/actions/runs/29852271609), branch head `22acea37b1d888dad3ce6b40f4c46c477c318a1a`.

## Resultado

| Plataforma | Archivo completo | Comprimido | Instalado | Archivos | SHA-256 | Auditoría |
|---|---|---:|---:|---:|---|---|
| Windows x86_64 | `Thappy-Windows-x86_64-0.1.0.zip` | 29,953,229 B (28.57 MiB) | 51,170,140 B (48.80 MiB) | 1,017 | `6796a8de5e93d6754a47360ec657d60cf36a8e1d17e42fc947735df7580bfc17` | OK, 0 errores, 0 warnings |
| Linux x86_64 | `Thappy-Linux-x86_64-0.1.0.tar.zst` | 28,563,765 B (27.24 MiB) | 54,417,955 B (51.90 MiB) | 1,017 | `e45620d6e035da108eba5bbcbbee927ba6d08378c60942283745acd3a45a65fc` | OK, 0 errores, 0 warnings |

Ambos pasan el límite de 104,857,600 bytes. El artefacto histórico medido era
171,072,354 B comprimido y 763,987,388 B extraído. La reducción es 82.49%/93.30%
en Windows y 83.30%/92.88% en Linux, respectivamente.

## Binarios y componentes

| Binario | Tamaño | SHA-256 | Formato verificado |
|---|---:|---|---|
| `Thappy.exe` | 19,725,824 B | `99b8d7922a435930f498eae3fcef2e24725929a06c156ddeb1913765eadb543d` | PE32+ GUI x86-64, MSVC 19.51 |
| `ThappyLauncher.exe` | 6,327,296 B | `77af9f67535cf8e615d24b612e25d5ac1042965e2bedabc0b3fc1648b6bfe3fb` | PE32+ console x86-64 |
| `thappy` | 23,169,912 B | `d9db3027d1628fcbae8ee14857be5beeb539a79c90d129be901a8c6a98bd65c6` | ELF x86-64 Release, stripped por CI |
| `thappy-launcher` | 6,135,960 B | `75efc8de268f4e4aec61cd74deb3a795dc51b81c67786d32d41f442479b97f06` | ELF x86-64, Go `-trimpath -s -w` |

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
- el ZIP Windows se extrajo en una carpeta vacía y `file` confirmó ambos PE64;
- el job Windows registró `cl.exe` y C/C++ `MSVC 19.51.36248.0`.

La auditoría verifica contenido y layout portable. No afirma boot gráfico ni
gameplay; esas pruebas están bloqueadas por los assets descritos en
`compatibility-test.md`.
