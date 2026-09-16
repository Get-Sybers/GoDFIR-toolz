# `get-sybers/goappcompat` — ShimCache (AppCompatCache) parser

Static Go binary on Velociraptor's `regparser` (and its `appcompatcache`
subpackage). Reads the AppCompatCache
(ShimCache) value from a SYSTEM hive and emits one record per entry — ControlSet,
CacheEntryPosition, Path, LastModifiedTimeUTC, SourceFile — as CSV or JSONL.
Executed/Duplicate state is not emitted (regparser's shimcache parser does
not expose it — never faked). `-d` finds hives by their
`regf` signature; the pipeline calls `-f /in/SYSTEM`. Parse-verified on a real
SYSTEM hive (373 shimcache entries — real system32 executable paths + last-mod
times).

**Dirty-hive .LOG replay:** when `SYSTEM.LOG1/.LOG2` sit alongside the hive,
goappcompat recovers a copy (applies the journalled dirty pages via
`regparser.RecoverHive`) into `--work-dir` and parses that. The recovered copy needs a **writable** work dir — the rootfs is
read-only, so mount a tmpfs and point `--work-dir` at it (`--tmpfs /tmp:...`; the
godfir-toolz lane wires this, like wxtcmd). No logs / unwritable work dir / recovery
error → it falls back to the committed hive with a one-line note (never
hard-fails).

```sh
docker build -t get-sybers/goappcompat:latest -f goappcompat/Dockerfile goappcompat
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only --tmpfs /tmp:rw,nosuid,nodev,size=256m \
  -v "$PWD/in:/input:ro" -v "$PWD/out:/output" \
  get-sybers/goappcompat:latest -f /input/SYSTEM --csv /output --csvf appcompatcache.csv
```
