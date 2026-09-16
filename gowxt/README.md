# `get-sybers/gowxt` — gowxt (replaces WxTCmd)

Static Go binary on `modernc.org/sqlite` (pure Go, no cgo). Reads the
Windows Timeline **ActivitiesCache.db** and emits WxTCmd's Activity
columns — the executable (from the `AppId` JSON), DisplayText / ContentInfo (from
the `Payload` JSON), the Start/End/LastModified/Expiration timestamps
(ActivitiesCache stamps Unix seconds → RFC3339 UTC), Duration and ActivityType —
as CSV or JSONL. It covers the `Activity` table (the timeline core); columns WxTCmd
derives from other providers, or that a given Windows build's schema doesn't
carry, are omitted, never faked.

SQLite needs a writable working area, but the input is mounted read-only under a
read-only rootfs, so gowxt copies the DB into `--work-dir` (a **tmpfs**), falling
back to an immutable read-only open when that dir isn't writable.

```sh
docker build -t get-sybers/gowxt:latest -f gowxt/Dockerfile gowxt
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only --tmpfs /work:rw,nosuid,nodev,uid=2000,gid=2000 \
  -v "$PWD/in:/input:ro" -v "$PWD/out:/output" \
  get-sybers/gowxt:latest -f /input/ActivitiesCache.db --csv /output --work-dir /work
```
