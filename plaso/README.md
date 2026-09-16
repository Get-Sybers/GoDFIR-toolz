# `get-sybers/plaso` — minimal hardened Plaso

Minimal hardened [Plaso](https://github.com/log2timeline/plaso) at a pinned
PyPI release (`PLASO_VERSION`), with three entry tools (`log2timeline.py`,
`psort.py`, `image_export.py`) plus the psort wrapper
(`psort_wrapper.py`, which imports a mounted custom output module so psort
discovers it, then hands psort the remaining argv verbatim). Plaso IS python,
so python and a shell remain and there is no single ENTRYPOINT — the caller
passes the full tool argv.

```sh
docker build -t get-sybers/plaso:latest -f plaso/Dockerfile .
```

Builds with the **repo root as context** so `COPY hardening/harden.yml`
consumes the canonical hardener directly; `get-sybers/*` namespace and
[the hardening contract](../README.md#the-hardening-contract) like every
other image here.
