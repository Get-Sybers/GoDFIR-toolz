# `get-sybers/gosbe` — gosbe (replaces SBECmd)

Static Go binary on `regparser`. Walks the **BagMRU** tree in
`NTUSER.DAT` / `UsrClass.dat` (all the Shell / ShellNoRoam roots), decodes the
shell items, reconstructs each shellbag's `AbsolutePath`, and emits one record
per shellbag — `BagPath`, `Slot`, `NodeSlot`, `MRUPosition`, `ShellType`,
`Value`, `AbsolutePath`, `LastWriteTime` — as JSONL.

Shell-item decoding is faithful for the common types: `0x1F` root/GUID folders
(mapped to known-folder names, e.g. *My Computer*, *Downloads*), `0x2F` volumes
(drive letters), and `0x30-0x3F` file/directory entries (the `BEEF0004`
extension's Unicode long name, with the ANSI short name as fallback). Honest
coverage gap: other shell-item types (property/delegate `0x00`, network
`0x40-0x4F`, URI `0x61`, …) are emitted with their `ShellType` and hex value but
**no reconstructed name** — never an invented path.

Parse-verified end to end on a real image (`rolf_long`, extracted
`Users/patcher/…/UsrClass.dat`, `.LOG1/.LOG2` replayed): gosbe reconstructed 36
shellbags — the user's browse trail including `C:\Users\patcher\Downloads\
survey.zip`, `…\AppData\Roaming\Wondershare\Wondershare Filmora`, a mapped
`Z:\ls_evidence` evidence drive, and the `Start Menu\Programs\Startup` folder.

```sh
docker build -t get-sybers/gosbe:latest -f gosbe/Dockerfile gosbe
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only --tmpfs /work:rw,nosuid,nodev,uid=2000,gid=2000 \
  -v "$PWD/in:/input:ro" -v "$PWD/out:/output" \
  get-sybers/gosbe:latest -d /input --json /output \
  --jsonf SBECmd_Output.json --work-dir /work
```

**Dirty hives:** a dirty hive needs its transaction LOGs (`.LOG1`/`.LOG2`)
extracted alongside. They are replayed into a recovered copy under
`--work-dir`, which must be a WRITABLE tmpfs (the rootfs is read-only) —
without one, replay falls back to the committed hive with a stderr note
rather than aborting.
