# godfir-tool/ — one parameterized Dockerfile for the remaining .NET tool images (`get-sybers/<tool>`)

`godfir-tool/Dockerfile` fetches a published .NET tool release at build time (a
recipe, not a committed binary — the upstream tools are MIT-licensed; see the
[licence note](../README.md#license)), verifies an optional SHA-256 pin, bakes
it into the official .NET runtime, runs the shared Ansible hardener
(`hardening/harden.yml`), then strips Ansible, apt, pip, sudo, every shell and
python itself out of the final image. The tool DLL is the pinned ENTRYPOINT.
The DLL inside each zip is located case-insensitively (release zip layouts and
casing vary — a tool may ship its DLL in a differently-cased subdirectory, or
lowercase like `rla.dll`), and the run-as uid/gid honour the
`DFIR_UID`/`DFIR_GID` build args.

The five images built this way: `sqlecmd` (its `Maps/` ruleset ships in the
image), `bstrings`, `recentfilecacheparser`, `rla` and `iisgeolocate`. Four
Windows-bound release binaries refuse to parse off-Windows; the Dockerfile
fails fast if asked to build one (the Go parsers cover those artefact classes
— `--build-arg GODFIR_TOOL_ALLOW_WINDOWS_ONLY=1` overrides, e.g. to unpack a
release).

## Contract

The built images are driven by the tool's own argv
([`contract.yml`](contract.yml): `entrypoint: argv`, one entry per built image
under `images:`) — a declared deviation until each tool gains an env→argv shim
or a Go port. They read no `GODFIR_TOOL_*` variables. Every built image
carries the label set with `com.get-sybers.tool=godfir-tool` (the recipe) and
`com.get-sybers.godfir-tool.name=<GODFIR_TOOL>` (the tool), and declares
`shell=false python=false static_binary=false` in `/etc/dfir-hardened`.

## Input

The file or directory named by the tool's `-f`/`-d`, mounted read-only
(`/input` by convention).

## Output

The directory named by the tool's `--csv`/`--json`/`--out` (`/output` by
convention); `bstrings` prints to stdout. There is no JSON summary line.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | the tool's own error |
| 2 | unused — declared because the framework's uniform table requires it; the built tools never exit 2 |

## Run

```sh
docker build -t get-sybers/sqlecmd:latest  --build-arg GODFIR_TOOL=SQLECmd  -f godfir-tool/Dockerfile .
docker build -t get-sybers/bstrings:latest --build-arg GODFIR_TOOL=bstrings -f godfir-tool/Dockerfile .
# pin the release:
docker build -t get-sybers/sqlecmd:latest  --build-arg GODFIR_TOOL=SQLECmd \
  --build-arg GODFIR_TOOL_SHA256=<sha256 of SQLECmd.zip> -f godfir-tool/Dockerfile .
# or everything at once:
./build-all.sh

docker run --rm --network none --read-only --tmpfs /tmp:rw,uid=2000,gid=2000 \
  --cap-drop ALL --security-opt no-new-privileges \
  -v "$PWD/in:/input:ro" -v "$PWD/out:/output" \
  get-sybers/sqlecmd:latest -f /input/History --csv /output
```

`test/contract_test.sh` builds one image (`GODFIR_TOOL`, default `SQLECmd`)
and asserts the label set, the self-declaration against the filesystem, and
that the entrypoint runs.

**iisGeolocate run note:** keep its MaxMind `.mmdb` databases current — mount
them read-only over the baked copies if the release's are stale.
