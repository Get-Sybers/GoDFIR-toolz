# `get-sybers/zeek` — Zeek LTS for offline capture parsing

Zeek LTS stripped to the zeek binary and its scripts (zkg/zeekctl removed),
fronted by **zeek-run**, a static Go batch entrypoint. `zeek-lts` comes from
the OpenSUSE `security:zeek` OBS repo, fetched from the master server
(`downloadcontent.opensuse.org`, direct content, no mirror redirect) with the
package version pinned (`ZEEK_VERSION`) and the repo signing key verified
against a pinned fingerprint before it is trusted (`ZEEK_KEY_FPR`,
fail-closed). Ansible (the build-time hardener), python and every shell are
removed from the runtime stage; glibc stays for zeek.

The image is self-orchestrating and env-driven: with no arguments zeek-run
reads the environment contract in [`contract.yml`](contract.yml), runs zeek
once per capture under the input tree, and prints one JSON summary line.

## Input

`ZEEK_INPUT_DIR` (default `/input`, mounted read-only) is walked recursively;
every file with a `.pcap`/`.pcapng`/`.cap` extension or a pcap/pcapng magic
number is one item.

## Env

| Variable | Default | Meaning |
|---|---|---|
| `ZEEK_INPUT_DIR` | `/input` | capture tree, recursed |
| `ZEEK_OUT_DIR` | `/output` | output root, one folder per capture |
| `ZEEK_WORK_DIR` | `/work` | scratch (writable tmpfs); zeek-run needs none but honours it |
| `ZEEK_FORCE` | `0` | `1/true/yes/on`: rerun captures that already have valid output |
| `ZEEK_FORMAT` | `json` | `json`: LogAscii JSON with ISO-8601 timestamps, logs renamed `*.json`; `tsv`: zeek's default tab-separated `*.log` |
| `ZEEK_SCRIPTS` | *(empty)* | space-separated extra zeek argv appended after `-C -r <capture>`: scripts to load and option assignments, e.g. `local Log::default_rotation_interval=0sec` |
| `ZEEK_LOG_LEVEL` | `info` | `error|warn|info|debug`, stderr only |

Each capture is parsed with `zeek -C -r <capture> [LogAscii::use_json=T
LogAscii::json_timestamps=JSON::TS_ISO8601] <ZEEK_SCRIPTS>` from inside its
output folder; zeek's own stdout/stderr go to the container's stderr.

## Output

Per capture, `<OUT_DIR>/<item>/<log>.json` (or `<log>.log` with
`ZEEK_FORMAT=tsv`) — every log zeek produced (`conn`, `dns`, `http`, `files`,
`packet_filter`, …) — plus `<OUT_DIR>/<item>/zeek.jsonl`, the index of
produced logs and their record counts (`{"log":"conn.json","records":N}` per
line). `<item>` is the capture path relative to `ZEEK_INPUT_DIR` with path
separators and whitespace folded to `_`. The index is written last and marks
the item done: a capture whose index exists is skipped on the next run unless
`ZEEK_FORCE` is set, and a forced rerun clears the previous logs first.

stdout is exactly one JSON object: `tool`, `version`, `status`, `inputs`,
`processed`, `skipped`, `failed`, `records` (log records across all captures),
`outputs`, `exit`, `started`, `duration_s`, plus `failures` (item + error)
when something failed and `error` on a config error. Progress, zeek's own
output and errors go to stderr.

## Exit codes

| Code | Status | Meaning |
|---|---|---|
| 0 | `ok` | every capture processed, or already up to date |
| 1 | `nothing` | no capture found, or every capture failed |
| 2 | `config_error` | bad variable, missing or unreadable input, unwritable output |
| 3 | `partial` | at least one capture processed and at least one failed |

## Run

```sh
docker build -t get-sybers/zeek:latest -f zeek/Dockerfile .
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only --tmpfs /tmp:rw,uid=2000,gid=2000 --tmpfs /work:rw,nosuid,nodev,uid=2000,gid=2000 \
  -v "$PWD/pcaps:/input:ro" -v "$PWD/out:/output" \
  -e ZEEK_FORMAT=json \
  get-sybers/zeek:latest
```

Builds with the repo root as context so `COPY hardening/harden.yml` consumes
the canonical hardener directly. `test/contract_test.sh` builds the image,
runs it over `test/fixtures/` (a generated one-packet DNS capture;
`test/gen_fixtures.py` remakes it), and asserts the summary line, the exit
code, idempotency and the config-error exit.

## argv pass-through (debug only)

Any argument is handed to the zeek binary verbatim with zeek's own exit code,
so `… get-sybers/zeek -C -r /input/x.pcap LogAscii::use_json=T` behaves
exactly like calling zeek; `--version` prints the entrypoint's version and
`--print-contract` prints `contract.yml`.
