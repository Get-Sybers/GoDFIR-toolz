# `get-sybers/gore` — gore (replaces RECmd)

Like the other registry tools, RECmd parses on Linux under .NET; this substitute
drops the .NET runtime with a static Go binary on Velociraptor's `regparser`. It
reads a **batch file** (`.reb` YAML: a list of keys with `HiveType`, `Category`,
`KeyPath`, `ValueName`, `Recursive`, `Comment`), walks each requested key in each
hive under `-d` (hives content-detected by their `regf` header, `HiveType`
inferred from the file name; `.LOG*` files skipped), and emits one record per
value — `HivePath`, `HiveType`, `Category`, `Description`, `Comment`, `KeyPath`,
`ValueName`, `ValueType`, `ValueData`, `LastWriteTimestamp`, `Recursive`,
`Deleted` — as JSONL or CSV, the exact shape byakugan's `recmd_batch` map
consumes.

The bundled `/batch/default.reb` is a **curated** forensic-key set (Run/RunOnce,
TypedPaths, ComputerName/TimeZone, …), **not** Eric Zimmerman's `Kroll_Batch.reb`
(which is not redistributable here) — supply your own with `--bn`. Honest
coverage gaps vs .NET RECmd: no RECmd **plugins** (the derived-value transforms),
no **deleted-cell recovery** (`Deleted` is always false), and the batch is the
curated set above rather than the full Kroll batch.

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
  --jsonf RECmd_Batch_Output.json --work-dir /work
```

**Dirty hives:** a dirty hive needs its transaction LOGs (`.LOG1`/`.LOG2`)
extracted alongside. They are replayed into a recovered copy under
`--work-dir`, which must be a WRITABLE tmpfs (the rootfs is read-only) —
without one, replay falls back to the committed hive with a stderr note
rather than aborting.
