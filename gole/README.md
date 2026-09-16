# `get-sybers/gole` — shell link (`.lnk`) parser

Static Go binary on `parsiya/golnk`. Parses Windows Shell Link (`.lnk`) files and emits one record per shortcut in an
operator-friendly CSV shape — target `Created`/`Modified`/`Accessed`, `FileSize`,
`LocalPath`, `RelativePath`, `WorkingDirectory`, `Arguments`, `IconLocation`,
`CommonPath`, and the decoded `HeaderFlags`/`FileAttributes` sets — as JSONL or
CSV. `-d` content-detects `.lnk` by the `0x4C` Shell Link header, so Plaso's
`$→_` image_export rename doesn't hide them.

Verified on real evidence: all 10 `.lnk` recovered from an actual host image
(`Users/patcher/…/Recent/`) parsed with zero failures, recovering the true target
paths (`C:\Users\patcher\Downloads\survey.zip`, `…\gen_2.py`), target timestamps
and flag sets.

```sh
docker build -t get-sybers/gole:latest -f gole/Dockerfile gole
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only -v "$PWD/in:/input:ro" -v "$PWD/out:/output" \
  get-sybers/gole:latest -d /input --csv /output --csvf lnk.csv
```
