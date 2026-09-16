# The all-in-one image (`eztools-all/`)

One image, every Linux-viable EZ tool, selected at **run** time — the
practical version of "the container adapts to the parser it's run with".
Hardening stays at build time (an immutable, read-only, root-less container
cannot meaningfully harden itself at runtime); what varies per run is which
parser the static Go launcher (`eztools-all/launcher/`) executes: the first
argument picks the tool case-insensitively, the rest is passed through, and
nothing else in the image is reachable via the entrypoint.

```sh
docker build -t get-sybers/eztools:latest -f eztools-all/Dockerfile .

docker run --rm get-sybers/eztools:latest list
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only --tmpfs /tmp -v "$PWD/in:/input:ro" -v "$PWD/out:/output" \
  get-sybers/eztools:latest EvtxECmd -d /input --csv /output
```

Why you'd want it over 15 per-tool images: one tag to pull, save and load for
offline/air-gapped use (one ~450 MB artefact instead of 15 × ~300 MB tars —
`docker save` only dedups shared base layers when you save all tags in a
single archive), one image warm in the cache across every lane, and per-tool
release pinning stays available via `eztools-all/checksums.sha256`.

**WxTCmd note** (applies to the per-tool image too): its SQLite interop
unpacks a native library beside the tool DLL, which a read-only rootfs
forbids. The launcher handles this — for WxTCmd it copies the tool to `/tmp`
and execs the copy — so run WxTCmd with a writable, exec-capable tmpfs:

```sh
  --tmpfs /tmp:rw,nosuid,nodev,exec,uid=2000,gid=2000,size=256m
```

(`EZTOOL_RUN_FROM_TMP=1|0` forces the behaviour on/off for any tool.)
