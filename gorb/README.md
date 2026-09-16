# `get-sybers/gorb` — gorb (replaces RBCmd)

Unlike PECmd/SrumECmd/SumECmd, RBCmd *does* parse on Linux under .NET — this
substitute exists to drop the .NET runtime, not to work around a Windows-only guard (the
`$I` metadata format is simple and fully specified, so a static Go binary is a
clean win; DX_DFIR #188 initiative 2). It parses the modern Recycle Bin `$I`
records — v1 (Vista–8.0, fixed 260-wchar path) and v2 (Win8.1/10/11,
length-prefixed path) — and emits RBCmd's columns (`SourceName`, `FileType`,
`FileName`, `FileSize`, `DeletedOn`) as CSV or JSONL. The legacy XP `INFO2`
container is not handled (obsolete, not in the pipeline's extraction filter).

`-d` finds records by their header, not their filename, so it picks up both a
raw-mount `$IXXXX` and Plaso's `image_export` rename (`$` → `_`, i.e. `_IXXXX`) —
the form the zimmerman lane actually feeds it. Parse-verified end to end on real
evidence: extracting `$Recycle.Bin` from a real acquisition with `image_export`
and running this image over the result recovers the deleted-file path, size and
deletion time.

```sh
docker build -t get-sybers/gorb:latest -f gorb/Dockerfile gorb
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only -v "$PWD/in:/input:ro" -v "$PWD/out:/output" \
  get-sybers/gorb:latest -d /input --csv /output --csvf gorb.csv
```
