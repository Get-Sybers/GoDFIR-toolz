# Changelog

All notable changes to `get_sybers.godfir_run` are documented here
([Keep a Changelog](https://keepachangelog.com/en/1.1.0/)).

## [0.1.0] - 2026-09-29
### Added
- Initial collection: the container-interaction roles moved out of DX_DFIR into
  the repo that owns the images (ansible-standards §1) — `godfir_run` (confined
  docker-run engine), `godfir_images` (image lifecycle, pulls the published
  images from the registry) and one lane role per image.
