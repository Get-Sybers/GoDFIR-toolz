# eztool/ — one parameterized Dockerfile for the remaining .NET tool images (`get-sybers/<tool>`)

`eztool/Dockerfile` fetches a published .NET tool release at build time (a
recipe, not a committed binary — the upstream tools are MIT-licensed; see the
[licence note](../README.md#license)), verifies an optional SHA-256 pin, bakes
it into the official .NET runtime, runs the shared Ansible hardener
(`hardening/harden.yml`), then strips Ansible, apt, pip, sudo, every shell and
python itself out of the final image. The tool DLL is the pinned ENTRYPOINT.
The DLL inside each zip is located case-insensitively (release zip layouts and
casing vary — a tool may ship its DLL in a differently-cased subdirectory, or
lowercase like `rla.dll`), and the run-as uid/gid honour the
`DFIR_UID`/`DFIR_GID` build args the DX_DFIR image role passes.

The five images still built this way, and their status:

- `sqlecmd` — parse-verified; its `Maps/` ruleset ships in the image
- `bstrings`, `recentfilecacheparser` — build-verified; parse-verify on first use
- `rla` — build-verified (its Registry library's `.LOG` replay is already
  proven on Linux by goamcache/goappcompat)
- `iisgeolocate` — build-verified; see the run note below

Four Windows-bound release binaries refuse to parse off-Windows; the
Dockerfile fails fast if asked to build one (the pipeline's own Go parsers
cover those artefact classes — `--build-arg EZTOOL_ALLOW_WINDOWS_ONLY=1`
overrides, e.g. to unpack a release).

```sh
docker build -t get-sybers/sqlecmd:latest  --build-arg EZTOOL=SQLECmd  -f eztool/Dockerfile .
docker build -t get-sybers/bstrings:latest --build-arg EZTOOL=bstrings -f eztool/Dockerfile .
# pin the release:
docker build -t get-sybers/sqlecmd:latest  --build-arg EZTOOL=SQLECmd \
  --build-arg EZTOOL_SHA256=<sha256 of SQLECmd.zip> -f eztool/Dockerfile .
# or everything at once:
./build-all.sh
```

**iisGeolocate run note:** keep its MaxMind `.mmdb` databases current — mount
them read-only over the baked copies if the release's are stale.
