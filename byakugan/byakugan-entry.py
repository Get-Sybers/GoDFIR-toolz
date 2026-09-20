#!/usr/bin/python3
"""The image's `byakugan` ENTRYPOINT: the engine's own multi-tool dispatcher,
byakugan.cli (build | timeline | verify | car-vocab — env-driven, one JSON
summary line, the uniform exit table; see contract.yml). The sub-tool set and
the input-layout discovery live in the engine, cloned at BYAKUGAN_REF and run
from its baked source tree (PYTHONPATH=/opt/byakugan); this file only
delegates."""
from byakugan.cli import main

if __name__ == "__main__":
    raise SystemExit(main())
