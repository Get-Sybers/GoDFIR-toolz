# `get-sybers/goevtx` — goevtx (replaces EvtxECmd)

Static Go binary on Velociraptor's `go-evtx`. Parses `.evtx` and emits one
JSON record per event in the EvtxECmd `*.json` shape that the DX_DFIR evtx
lane and byakugan's winevt/evtx maps consume — `EventId`,
`Provider`, `Channel`, `Computer`, `EventRecordId`, `TimeCreated`, `Level`,
`UserId`, and `Payload` (the event's EventData rendered as the classic
`{"EventData":{"Data":[{"@Name","#text"}...]}}` form, or `{"UserData":...}`),
plus `SourceFile` and a null `MapDescription`. It does **not** reproduce
EvtxECmd's Maps layer (the per-provider YAML deriving `PayloadData1-6` /
`MapDescription`) — byakugan reads the raw EventData, not those derived columns,
so the substitute is faithful to what the pipeline consumes; those fields are
omitted, never faked. The `--xml` sidecar is a best-effort reconstruction for
manual review (not the original binary XML, and not ingested).

Parse-verified end to end on a real Sysmon `.evtx`: goevtx's output fed straight
through byakugan's `evtx_sysmon` map yielded correct CAR `process/create` and
`process/terminate` events (exe, pid/ppid, command line, integrity level,
SHA-256, ProcessGuid, parent links) for all 85 events.

```sh
docker build -t get-sybers/goevtx:latest -f goevtx/Dockerfile goevtx
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only -v "$PWD/in:/input:ro" -v "$PWD/out:/output" \
  get-sybers/goevtx:latest -f /input/Security.evtx --json /output \
  --jsonf Security_EvtxECmd_Output.json --xml /output --xmlf Security_EvtxECmd_Output.xml
```
