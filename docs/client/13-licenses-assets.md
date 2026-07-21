# Licencias y assets

## Código

OTClient usa MIT. El paquete debe incluir:

- `LICENSE`;
- `THIRD_PARTY_NOTICES.md`;
- `licenses/OTClient-MIT.txt`.

Las atribuciones upstream se conservan. Los nombres y gráficos de OpenTibiaBR no se presentan como propiedad de Thappy.

## Dependencias

`vcpkg.json` fija el baseline y enumera las dependencias C++. La release debe adjuntar SBOM y evidencia de licencias resuelta durante CI. El launcher usa la biblioteca estándar de Go para reducir dependencias de distribución.

## Assets de cliente

`data/things/1525` y `data/sounds/1525` no están comprometidos en este repositorio. No se publican sprites, catálogos ni sonidos mientras no exista autorización de redistribución comprobable.

El paquete puede incluir imágenes, fuentes y estilos ya distribuidos por el repositorio MIT cuando no tengan una licencia separada incompatible. Cualquier archivo con licencia propia prevalece; por ejemplo, `modules/game_wheel/LICENSE` debe acompañar ese módulo si se distribuye. El perfil production no distribuye ese módulo visual.

## Marca temporal

El icono y background upstream sólo pueden funcionar como placeholders identificados. La primera release no debe describirlos como identidad final de Thappy.
