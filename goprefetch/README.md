# `get-sybers/goprefetch` — goprefetch (replaces PECmd)

Static Go binary on Velociraptor's `go-prefetch`, whose pure-Go
LZXpress-Huffman implementation decompresses Win8+/Win10/Win11 MAM prefetch on
any OS. Verified in this repo against real fixtures: WinXP, Vista, Win8.1,
Win10 and Win11 `.pf` files — all four MAM-compressed samples included — parse
correctly on Linux.

```sh
docker build -t get-sybers/goprefetch:latest -f goprefetch/Dockerfile goprefetch
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only -v "$PWD/in:/input:ro" -v "$PWD/out:/output" \
  get-sybers/goprefetch:latest -d /input --json /output
```

JSONL (or `--csv`) per file: `SourceFilename`, `Executable`, `Path`, `Hash`,
`Version`, `FileSize`, `RunCount`, `LastRun`, `PreviousRuns`,
`FilesAccessed`. Volume info blocks are the one PECmd output section not
emitted (not exposed by the library).

**Why it exists:** PECmd remains the reference parser *on Windows*;
`get-sybers/goprefetch` exists because Linux pipelines otherwise had to fall
back to Plaso for `.pf`.
