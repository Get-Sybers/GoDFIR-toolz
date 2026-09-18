# `get-sybers/gojle` — jump list (AutomaticDestinations) parser

Static Go binary reading AutomaticDestinations (`*.automaticDestinations-ms`,
an OLE compound file, via `richardlehane/mscfb`) and their `DestList` stream. It
emits one record per jump-list file in the AutomaticDestinations record shape
that byakugan's `jlecmd_dest` map consumes: `AppId`
(with the well-known friendly name), `SourceFile`, and the per-target
`DestListEntries` — `Path`, `EntryNumber`, `CreatedOn` (recovered from each
entry's embedded LNK stream), `LastModified`, `Hostname`, `InteractionCount`,
`MRUPosition`, `Pinned`, `MacAddress` (from the FileDroid GUID node) and
`VolumeDroid`. DestList versions 1/3/4 are handled; CustomDestinations files are
skipped, never mis-parsed (byakugan consumes the AutomaticDestinations shape).

Input is one of three modes: a single file (`-f`), a directory scanned recursively
(`-d`), or a tar archive on stdin (`--tar`) as `gomount stream` emits — one entry
per file, entry name = the file's volume path. The tar mode selects the
AutomaticDestinations entries by name, buffers each whole (the OLE reader needs
random access) and runs the same DestList parse the file modes use, one file in
memory at a time. A per-file parse failure is counted and skipped; the stream
keeps going. The orchestration is a pipe:

```sh
gomount stream --filter '*.automaticDestinations-ms' disk.E01 | gojle --tar
```

Verified end to end on real evidence: the 6 AutomaticDestinations jump lists from
an actual host image fed straight through byakugan's `jlecmd_dest` map yielded 14
correct CAR `file/read` events — real target paths, `LastModified` timestamps,
hostname `desktop-b2lequd`, pinned/known-folder flags and the creating host's MAC.

```sh
docker build -t get-sybers/gojle:latest -f gojle/Dockerfile gojle
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only -v "$PWD/in:/input:ro" -v "$PWD/out:/output" \
  get-sybers/gojle:latest -d /input --json /output --jsonf jumplists.json
```
