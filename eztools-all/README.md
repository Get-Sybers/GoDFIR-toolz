# The all-in-one image (`eztools-all/`)

One image, every remaining .NET tool, selected at **run** time — the
practical version of "the container adapts to the parser it's run with".
Hardening stays at build time (an immutable, read-only, root-less container
cannot meaningfully harden itself at runtime); what varies per run is which
parser the static Go launcher (`eztools-all/launcher/`) executes: the first
argument picks the tool case-insensitively, the rest is passed through, and
nothing else in the image is reachable via the entrypoint.

```sh
docker build -t get-sybers/eztools:latest -f eztools-all/Dockerfile .

docker run --rm get-sybers/eztools:latest list       # prints the tool names
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only --tmpfs /tmp -v "$PWD/in:/input:ro" -v "$PWD/out:/output" \
  get-sybers/eztools:latest <Tool> -d /input --csv /output
```

Why you'd want it over separate per-tool images: one tag to pull, save and
load for offline/air-gapped use (one ~450 MB artefact instead of a ~300 MB
tar per tool — `docker save` only dedups shared base layers when you save all
tags in a single archive), one image warm in the cache across every lane, and
per-tool release pinning stays available via `eztools-all/checksums.sha256`.

**Native-interop note** (applies to the per-tool images too): a tool whose
interop unpacks a native library beside its DLL cannot do so on the
read-only rootfs. The launcher handles this — it copies such a tool to
`/tmp` and execs the copy — so give those runs a writable, exec-capable
tmpfs:

```sh
  --tmpfs /tmp:rw,nosuid,nodev,exec,uid=2000,gid=2000,size=256m
```

(`EZTOOL_RUN_FROM_TMP=1|0` forces the behaviour on/off for any tool.)
