# `get-sybers/goprefetch` — Windows prefetch (`.pf`) parser

Static Go binary on Velociraptor's `go-prefetch`, whose pure-Go
LZXpress-Huffman implementation decompresses Win8+/Win10/Win11 MAM prefetch on
any OS. Verified in this repo against real fixtures: WinXP, Vista, Win8.1,
Win10 and Win11 `.pf` files — all four MAM-compressed samples included — parse
correctly on Linux. Mount the evidence read-only into the container and point
`-d` at it (or `-f` at a single file) — it walks the tree and content-detects
the `.pf` files.

```sh
docker build -t get-sybers/goprefetch:latest -f goprefetch/Dockerfile goprefetch
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only -v "$PWD/in:/input:ro" -v "$PWD/out:/output" \
  get-sybers/goprefetch:latest -d /input --json /output
```

## Reading a disk image over a pipe (`--tar`)

`--tar` reads a TAR archive on stdin — one entry per file, entry name = the
file's volume path, body = the file's bytes, exactly as `gomount stream` emits —
and parses every `*.pf` entry. A disk image is processed by a plain pipe, with
no mount and no intermediate extraction:

```sh
gomount stream --filter '*.pf' disk.E01 | goprefetch --tar --json /output
```

Each entry is buffered in memory (prefetch files are small), so the whole
archive is never held. A read or parse failure on one entry is logged on stderr
and counted; the stream continues. Exactly one of `-f`, `-d`, or `--tar` is
given per run.

JSONL (or `--csv`) per file: `SourceFilename`, `Executable`, `Path`, `Hash`,
`Version`, `FileSize`, `RunCount`, `LastRun`, `PreviousRuns`,
`FilesAccessed`. Volume info blocks are not emitted (not exposed by the
library).

