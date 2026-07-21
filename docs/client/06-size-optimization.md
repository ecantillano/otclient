# Optimización de tamaño

## Causa medida

El artifact anterior disponible tenía 728.60 MiB descomprimidos. PDB e ILK
sumaban 687.82 MiB y representaban 94.40% del contenido. El ZIP de Actions
medía 163.15 MiB, pero no era portable porque sólo contenía ejecutable, PDB e
ILK.

## Cambios aplicados

- build `Release` real en Windows y Linux;
- PDB, ILK, objetos, librerías estáticas, caches y build trees bloqueados por
  auditor;
- paquetes creados desde una allowlist de runtime;
- `mods/`, BOT y módulos visuales modernos excluidos;
- launcher separado del core;
- componentes `core`, `modules`, `data` y `assets` independientes;
- `assets` sólo se genera cuando existen archivos autorizados bajo
  `data/things/<version>/`, `data/sounds/<version>/` o catálogos aprobados;
- paquete Linux en `tar.zst` y componentes de transporte en ZIP para el
  extractor auditado del launcher;
- sin UPX ni packers.

No se descargan ni redistribuyen sprites o sonidos desde repositorios de
terceros. La ausencia de assets autorizados es visible y bloquea la prueba de
gameplay, pero no se oculta llenando el paquete con datos sin licencia.

## Budgets

No se fijó un límite arbitrario antes de medir. El primer build Release
validado establece el baseline por plataforma y componente. A partir de ahí:

- cada reporte registra bytes comprimidos y sin comprimir;
- CI rechaza crecimiento superior a 10% contra el baseline elegido;
- un presupuesto duro es opcional y debe provenir de una medición aprobada;
- assets se miden aparte para no ocultar regresiones de core/modules/data;
- el top 100 y el resumen por carpeta quedan en `size-report-<platform>.json`.

## Auditoría

`scripts/audit-client-package.py` verifica:

- top 100 por tamaño con SHA-256;
- tamaño comprimido y sin comprimir;
- archivos y directorios de build prohibidos;
- símbolos y referencias de debug en PE/ELF;
- DLL/SO duplicadas por basename;
- BOT y módulos modernos;
- endpoints privados o de prueba en production;
- configuración canónica del protocolo 1525;
- licencias obligatorias;
- layout portable y ejecutables esperados.

Los tamaños definitivos, budgets derivados y reducción contra el baseline se
registran en `14-final-status.md` usando los paquetes descargados del workflow,
no estimaciones del checkout.
