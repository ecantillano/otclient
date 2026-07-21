# Baseline de tamaño

Fecha: 2026-07-21  
Fuente: GitHub Actions run `29675622968`, artifact `windows-cmake-release`, ID `8439220333`.

## Resultado medido

| Métrica | Bytes | MiB |
|---|---:|---:|
| ZIP descargado por Actions | 171,072,354 | 163.15 |
| Contenido descomprimido | 763,987,388 | 728.60 |
| Ejecutable runtime | 42,760,192 | 40.78 |
| PDB + ILK | 721,227,196 | 687.82 |

No se encontró un ZIP literal de 626 MB. El artefacto disponible reproduce y explica el mismo problema reportado: un build `RelWithDebInfo` publicó símbolos y estado incremental. La cifra visible puede variar según si GitHub, Explorer o el usuario muestran bytes, MB decimales, MiB, ZIP o carpeta extraída.

## Contenido completo del artifact

Sólo existen tres archivos, por lo que esta tabla también es el top 100 completo.

| Ruta | Sin comprimir | Comprimido estimado real | % del total extraído | Runtime | Acción | Riesgo |
|---|---:|---:|---:|---|---|---|
| `otclient.pdb` | 457,019,392 B | 118,745,110 B | 59.82% | no | excluir del paquete público; conservar como artifact privado opcional | sin diagnóstico simbólico público |
| `otclient.ilk` | 264,207,804 B | 39,435,907 B | 34.58% | no | excluir siempre | ninguno en runtime |
| `otclient.exe` | 42,760,192 B | 12,890,967 B | 5.60% | sí | renombrar y empaquetar como `Thappy.exe` | ninguno |

PDB e ILK representan 94.40% del tamaño extraído y 92.47% del ZIP. El ejecutable solo comprime a 12.29 MiB.

## Hallazgos por categoría

| Categoría | Evidencia | Acción |
|---|---|---|
| Símbolos debug | PDB de 435.85 MiB | no publicar en paquetes de jugadores |
| Estado incremental | ILK de 251.97 MiB | no publicar |
| Fuentes | no estaban en este artifact | el auditor debe mantenerlo así |
| Build tree | no estaba completo, pero PDB/ILK son productos de build | bloquear por extensión |
| Runtime data | ausente | empaquetar explícitamente `init.lua`, `config/`, `data/`, `modules/` y mods permitidos |
| Assets 1525 | ausentes | componente separado; no redistribuir sin licencia confirmada |
| BOT | no estaba porque tampoco había recursos runtime | auditar el paquete portable nuevo, no inferir éxito desde este artifact |
| Duplicados | ninguno; sólo tres archivos | auditar DLL/SO en cada paquete futuro |

## El artifact anterior no era portable

No contiene `init.lua`, `data/`, `modules/` ni `mods/`. `otclient.exe` busca `init.lua` para descubrir el directorio de trabajo, por lo que esos tres archivos no forman una instalación limpia utilizable.

El ejecutable es PE32+ x86-64, subsystem Windows CUI y no tiene Authenticode. SHA-256:

```text
c53db8130939859766ddf8ef2b2ecbb763dd47c02f86627bd3d59423e09aea5c  otclient.exe
```

## Presupuesto derivado de la medición

No se fija un límite final antes del primer paquete portable. Los gates iniciales son:

- ejecutable y librerías: medir el primer Release real y bloquear +10%;
- modules: baseline del primer paquete sin BOT/UI moderna y bloquear +10%;
- data: baseline del primer paquete autorizado y bloquear +10%;
- assets: componente aparte, medido por asset version;
- launcher: baseline del primer binario Go y bloquear +10%;
- paquete completo: comparar contra el primer paquete portable, además de reportar la reducción contra 728.60 MiB.

La meta comprobable inmediata es retirar 687.82 MiB de productos de compilación y producir un paquete que arranque fuera del build tree.
