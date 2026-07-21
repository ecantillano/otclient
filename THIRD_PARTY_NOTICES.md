# Third-party notices

## OTClient Redemption

Thappy is based on OTClient and the OpenTibiaBR OTClient Redemption fork. OTClient is licensed under the MIT License. The required text is included in `licenses/OTClient-MIT.txt` and the repository root `LICENSE`.

The original authors and upstream projects retain their copyright and trademarks. Thappy does not claim OpenTibiaBR branding as its own.

## Runtime libraries

Desktop builds use libraries resolved by the pinned vcpkg manifest in `vcpkg.json`. The release workflow generates an SBOM and keeps each dependency's license metadata with CI evidence. A release must fail review if that evidence is missing.

## Client assets

This repository does not commit Tibia 1525 sprites, catalogs or sounds. They are excluded from public Thappy artifacts until their redistribution rights are confirmed. An empty component or a manifest pointer is not evidence of authorization.

## Thappy placeholders

Any reused upstream icon or background remains an upstream asset and is marked as a temporary placeholder in release notes. It is not a final Thappy brand asset.
