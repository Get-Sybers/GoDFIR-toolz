# `get-sybers/gore` — batch-driven registry dumper

Static Go binary on Velociraptor's `regparser`. Reads a **batch file** (`.reb` YAML: a list of keys with `HiveType`, `Category`,
`KeyPath`, `ValueName`, `Recursive`, `Comment`), walks each requested key in each
hive under `-d` (hives content-detected by their `regf` header, `HiveType`
inferred from the file name; `.LOG*` files skipped), and emits one record per
value — `HivePath`, `HiveType`, `Category`, `Description`, `Comment`, `KeyPath`,
`ValueName`, `ValueType`, `ValueData`, `LastWriteTimestamp`, `Recursive`,
`Deleted` — as JSONL or CSV, the exact shape byakugan's `recmd_batch` map
consumes.

The bundled `/batch/default.reb` is a **curated** forensic-key set (Run/RunOnce,
TypedPaths, ComputerName/TimeZone, …) — supply your own with `--bn`. Honest
coverage notes: no derived-value plugin transforms, and no **deleted-cell
recovery** (`Deleted` is always false).

Parse-verified end to end on a real image (`rolf_long`, extracted SYSTEM /
SOFTWARE / NTUSER.DAT with `.LOG1/.LOG2` replayed): gore's 18 records fed
straight through byakugan's `recmd_batch` map yielded 18 CAR `registry` /
`value_edit` events — including a real OneDrive Run-key persistence entry for
user `patcher`.

```sh
docker build -t get-sybers/gore:latest -f gore/Dockerfile gore
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only --tmpfs /work:rw,nosuid,nodev,uid=2000,gid=2000 \
  -v "$PWD/in:/input:ro" -v "$PWD/out:/output" \
  get-sybers/gore:latest -d /input --json /output \
  --jsonf registry_batch.json --work-dir /work
```

**Dirty hives:** a dirty hive needs its transaction LOGs (`.LOG1`/`.LOG2`)
extracted alongside. They are replayed into a recovered copy under
`--work-dir`, which must be a WRITABLE tmpfs (the rootfs is read-only) —
without one, replay falls back to the committed hive with a stderr note
rather than aborting.
