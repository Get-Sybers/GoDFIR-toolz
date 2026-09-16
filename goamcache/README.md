# `get-sybers/goamcache` — `Amcache.hve` parser

Static Go binary on Velociraptor's `regparser`. Parses an `Amcache.hve` and emits one record per program-execution file entry
(`Root\InventoryApplicationFile`) — the key's last-write time, ProgramId, the
SHA-1 (the `0000`-prefixed `FileId` stripped to the bare 40-hex hash), full path,
name, publisher/product/version and size — as CSV or JSONL. Dirty-hive `.LOG1/.LOG2` transaction logs **are
replayed** (`regparser.RecoverHive`) when they sit beside the hive; replay writes a recovered copy under `--work-dir`
(default `$TMPDIR`), which must be writable — mount a **tmpfs** there (the rootfs
is read-only). If the logs are absent or replay fails it falls back to the
committed hive with a stderr note (never a hard fail). Parse-verified on a real
Amcache.hve (237 entries — real program names, 40-hex SHA-1s, full paths and key
times; on this clean-shutdown image the `.LOG` replay was a no-op — 237 either way).

```sh
docker build -t get-sybers/goamcache:latest -f goamcache/Dockerfile goamcache
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only --tmpfs /work:rw,nosuid,nodev,uid=2000,gid=2000 \
  -v "$PWD/in:/input:ro" -v "$PWD/out:/output" \
  get-sybers/goamcache:latest -f /input/Amcache.hve --csv /output --csvf amcache.csv -i --work-dir /work
```
