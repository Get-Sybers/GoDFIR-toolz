# eztool/ — one parameterized Dockerfile for every .NET EZ tool (`get-sybers/<tool>`)

`eztool/Dockerfile` fetches the published .NET release at build time (a
recipe, not a committed binary — the tools are MIT-licensed), verifies an
optional SHA-256 pin, bakes it into the official .NET runtime, runs the shared
Ansible hardener (`hardening/harden.yml`), then strips Ansible, apt, pip,
sudo, every shell and python itself out of the final image. The tool DLL is
the pinned ENTRYPOINT. The DLL inside each zip is located case-insensitively
(release zip layouts and casing vary — EvtxECmd ships an `EvtxeCmd/` dir, rla
ships `rla.dll`), and the run-as uid/gid honour the `DFIR_UID`/`DFIR_GID`
build args the DX_DFIR image role passes.

```sh
docker build -t get-sybers/jlecmd:latest   --build-arg EZTOOL=JLECmd   -f eztool/Dockerfile .
docker build -t get-sybers/bstrings:latest --build-arg EZTOOL=bstrings -f eztool/Dockerfile .
# pin the release:
docker build -t get-sybers/sqlecmd:latest  --build-arg EZTOOL=SQLECmd \
  --build-arg EZTOOL_SHA256=<sha256 of SQLECmd.zip> -f eztool/Dockerfile .
# or everything at once:
./build-all.sh
```

**iisGeolocate run note:** keep its MaxMind `.mmdb` databases current — mount
them read-only over the baked copies if the release's are stale.
