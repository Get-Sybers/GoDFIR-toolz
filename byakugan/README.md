# `get-sybers/byakugan` — the Byakugan MITRE CAR engine

The external [Byakugan](https://github.com/Get-Sybers/byakugan) MITRE CAR
engine in one hardened python image (plus a static Go parse binary,
`byakugan-parse`). The engine is cloned **recursively** at build time at
`--build-arg BYAKUGAN_REF` — its nested `third_party/car` +
`attack-datasources` submodules rebuild the object model — so the consuming
DX_DFIR checkout holds nothing but how it invokes this image. DX_DFIR passes
its `sources.yml` pin; the default here is `main`.

The engine's `byakugan` console binary is the ENTRYPOINT
(`byakugan-entry.py`): one binary, dispatched on its first argument to the
engine's three operations (`build` / `timeline` / `car-vocab`).

```sh
docker build -t get-sybers/byakugan:latest \
  --build-arg BYAKUGAN_REF=<40-hex sha> -f byakugan/Dockerfile .
```

Builds with the **repo root as context** so `COPY hardening/harden.yml`
consumes the canonical hardener directly; `get-sybers/*` namespace and
[the hardening contract](../README.md#the-hardening-contract) like every
other image here (python3 stays — it is a python engine — everything else is
stripped).
