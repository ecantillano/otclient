# Changelog

## 0.1.0 - prerelease candidate

- Added the locked Thappy production environment for protocol 1525.
- Renamed the desktop client and executables to Thappy.
- Removed the integrated BOT, its profiles, scripts, UI and resources.
- Added the explicit retro runtime module profile while retaining protocol parsers.
- Added allowlisted migration for normal player settings and minimaps.
- Added a secure component launcher with SHA-256, offline policy and rollback.
- Added reproducible runtime packaging, audits, evals and draft-release workflows.
- Disabled third-party asset downloads; authorized 1525 assets remain external.

## 2026-02-04
- Added OTML alias resolution so `.otui` files can declare variables (e.g. `&primaryColor`) and reference them as `$primaryColor`, improving theme consistency and readability.

## 05-12-2023
### Breaking API Changes
- `UIWidget` property `qr-code` & `qr-code-border` replaced with `UIQrCode` properties `code` & `code-border`
- `image-source-base64` replaced with `image-source: base64:/path/to/image`
- `#include "shadermanager.h"` moved to `#include <framework/graphics/shadermanager.h>`
