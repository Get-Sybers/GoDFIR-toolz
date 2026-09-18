# `get-sybers/gorb` — Recycle Bin `$I` parser

Static Go binary. Parses the modern Recycle Bin `$I` records — v1 (Vista–8.0, fixed 260-wchar path) and v2 (Win8.1/10/11,
length-prefixed path) — and emits `SourceName`, `FileType`,
`FileName`, `FileSize` and `DeletedOn` as CSV or JSONL. The legacy XP `INFO2`
container is not handled (obsolete, not in the pipeline's extraction filter).

Input is one of three modes: `-f` a single file, `-d` a directory scanned
recursively, or `--tar` a tar archive on stdin.

`-d` finds records by their header, not their filename, so it picks up both a
raw-mount `$IXXXX` and Plaso's `image_export` rename (`$` → `_`, i.e. `_IXXXX`) —
the form the godfir-toolz lane actually feeds it. Parse-verified end to end on real
evidence: extracting `$Recycle.Bin` from a real acquisition with `image_export`
and running this image over the result recovers the deleted-file path, size and
deletion time.

```sh
docker build -t get-sybers/gorb:latest -f gorb/Dockerfile gorb
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only -v "$PWD/in:/input:ro" -v "$PWD/out:/output" \
  get-sybers/gorb:latest -d /input --csv /output --csvf gorb.csv
```

## `--tar`: consume a `gomount stream`

`--tar` reads a TAR archive on stdin — the stream `gomount stream` emits, one
entry per file with the entry name set to the file's volume path — and parses the
`$I` records out of it. Records are detected by header, the same as `-d`, so the
`$Recycle.Bin/*` glob's `$R` payloads and `desktop.ini` are skipped and only the
`$I` records are emitted. `SourceName` is the entry's volume path. The
orchestration is a pipe:

```sh
gomount stream --filter '$Recycle.Bin/*' disk.E01 | gorb --tar --csv /output --csvf gorb.csv
```
