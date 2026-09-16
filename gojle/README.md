# `get-sybers/gojle` — gojle (replaces JLECmd)

JLECmd parses on Linux under .NET; this substitute drops the .NET runtime with a
static Go binary that reads AutomaticDestinations (`*.automaticDestinations-ms`,
an OLE compound file, via `richardlehane/mscfb`) and their `DestList` stream. It
emits one record per jump-list file in the JLECmd AutomaticDestinations shape
byakugan's `jlecmd_dest` map / `jlecmd` adapter consume: `AppId`
(with the well-known friendly name), `SourceFile`, and the per-target
`DestListEntries` — `Path`, `EntryNumber`, `CreatedOn` (recovered from each
entry's embedded LNK stream), `LastModified`, `Hostname`, `InteractionCount`,
`MRUPosition`, `Pinned`, `MacAddress` (from the FileDroid GUID node) and
`VolumeDroid`. DestList versions 1/3/4 are handled; CustomDestinations files are
skipped, never mis-parsed (byakugan consumes the AutomaticDestinations shape).

Verified end to end on real evidence: the 6 AutomaticDestinations jump lists from
an actual host image fed straight through byakugan's `jlecmd_dest` map yielded 14
correct CAR `file/read` events — real target paths, `LastModified` timestamps,
hostname `desktop-b2lequd`, pinned/known-folder flags and the creating host's MAC.

```sh
docker build -t get-sybers/gojle:latest -f gojle/Dockerfile gojle
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only -v "$PWD/in:/input:ro" -v "$PWD/out:/output" \
  get-sybers/gojle:latest -d /input --json /output --jsonf JLECmd_Output.json
```
