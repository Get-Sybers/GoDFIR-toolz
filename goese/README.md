# `get-sybers/goese` — ESE database dumper (SRUM / SUM)

Static Go binary on Velociraptor's `go-ese` (pure-Go ESE). Verified in this
repo against a real 7.8 MB `SRUDB.dat`: all provider tables dumped (16k+ rows
in ApplicationResourceUsage), `SruDbIdMapTable` decoded automatically —
`AppId`/`UserId` columns gain `AppIdName`/`UserIdName` (UTF-16 strings, SIDs
for IdType 3), ESE DateTime columns arrive as RFC3339. Well-known SRUM
provider GUID tables get friendly output names (`ApplicationResourceUsage`,
`NetworkDataUsage`, `NetworkConnectivityUsage`, `EnergyUsage[LT]`,
`AppTimelineProvider`, `PushNotifications`); it reads SUM `Current.mdb` — or
any ESE database — the same way (`--list` shows tables).

```sh
docker build -t get-sybers/goese:latest -f goese/Dockerfile goese
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only -v "$PWD/in:/input:ro" -v "$PWD/out:/output" \
  get-sybers/goese:latest -f /input/SRUDB.dat --json /output
```

**Directory mode:** `goese -d /image` finds every SRUM database (`SRUDB.dat`)
and SUM database (`*.mdb` under a `SUM/` directory) case-insensitively and
dumps each into its own sub-directory (`SRUM_SRUDB/`, `SUM_Current/`, …) with
a `SourceDb` field on every row.
