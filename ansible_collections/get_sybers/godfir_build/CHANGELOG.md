# Changelog

All notable changes to `get_sybers.godfir_build` are documented here
([Keep a Changelog](https://keepachangelog.com/en/1.1.0/)).

## [0.1.0] - 2026-09-29
### Added
- The `verify` and `audit` entry points now delegate to
  `get_sybers.godfir_run.godfir_images` (one home for the runtime gate);
  the collection declares the `get_sybers.godfir_run` dependency.
- Initial collection: the `godfir_build` role and `build_images` playbook,
  moved from the repo root into the native `ansible_collections/get_sybers/`
  layout (ansible-standards §1).
