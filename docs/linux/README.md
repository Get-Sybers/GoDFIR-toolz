# GoDFIR-toolz for Linux — the plan

**Status:** plan. **Scope:** a Linux artefact matrix in this repository with the
same shape as the Windows one: static Go parsers, one per artefact class,
inside the [Container Framework](../framework/README.md), driven by DX_DFIR
lanes and mapped into CAR by byakugan. This paper states what gets built, in
what order, and which decisions are taken or still open. Framework
cross-references use the paper's `§<file>.<section>` notation, e.g.
[§3.5](../framework/03-environment-contract.md) is the exit-code table.

## Abstract

The Windows artefact classes are parsed by twelve static Go tools — `FROM
scratch`, env-contract driven, one JSON summary line — while Linux evidence is
still processed by Plaso: `log2timeline` parses it, and byakugan reaches it
only through `l2t_*` adapter maps. This plan replicates the Windows approach
for Linux with **no log2timeline dependency anywhere in the Linux path**. It
has three pillars:

1. **`pinfo`** — one shared Go module every parser imports (replacing the
   byte-identical `batch.go` copies): the batch runtime, a common record
   envelope, timestamp normalisation, content-first discovery, the
   `gomount stream` tar consumer, and run/output introspection. The Linux
   tools are born on it; the Windows tools adopt it in a mechanical
   follow-up phase.
2. **Fourteen Linux parsers** (`gojournal`, `goauditd`, `gowtmp`,
   `gosyslog`, `goshell`, `gousers`, `gocron`, `gounit`, `gotrash`,
   `gohost`, `gonetwork`, `goctl`, `gopkg`, `goacct`) — packages of one
   framework-conformant Tier-1 tool from their first commit — every one a
   **daemon parser** (§4): it reads what one daemon
   writes or is told — with record schemas designed against the byakugan
   data model
   (§4.1): the parsers extract everything byakugan's CAR maps need — typed
   rows, native vocabulary, identity fields, join keys — and derive nothing
   byakugan owns (relationships, canonicalisation, enrichment). The built
   matrix ships as ONE structured binary and image, `godaemonhunter`
   (decisions 15–16): every parser a package and sub-command of it, plus
   `hunt`, the layered Layer-1 → knowledge store → enriched-daemon-parser
   run — there are no standalone per-parser binaries.
3. **Linux evidence access in gomount** — continuing the native-extraction
   role gomount already started for Windows (`materialise` sets,
   `stream`→`--tar`): ext4/XFS/Btrfs (and vfat, squashfs)
   backends behind the existing verb surface, a volume stack that peels
   partition → mdraid → LUKS → LVM, new image formats (qcow2 beside raw/E01),
   a `linux-core` materialise set replacing Plaso `image_export` for Linux
   images, a `timeline` verb replacing the `l2t_filestat` role — and
   **snapshot traversal**: LVM and Btrfs snapshots (and qcow2 internal
   snapshots) are enumerated and passed through `stream`/`materialise`/
   `timeline` exactly as VSS stores are passed on the Windows side, with
   provenance carried into every record — and **filesystem residue**: what
   each filesystem leaves behind (`lost+found`, orphaned and deleted-but-
   present entries, directory slack names, fs journals) is surfaced through
   the same verbs with the same provenance discipline, and recovered
   content is fed back through the same parsers (§5.5).

Done means: a Linux disk image — plain, LVM, or Btrfs, snapshots included —
processes end-to-end into per-artefact JSONL and CAR **with the Plaso image
absent**, and every new image passes the framework gate.

> ## The rules
>
> **1. The Go toolz never replace byakugan, and never use its synthetic
> joining guids.** The parsers are parsers: they extract, complete and
> native, everything byakugan needs to do its job — and nothing more. CAR
> normalisation, canonicalisation, row identity, relationships and every
> derived view are byakugan's; the row guids (`guid`, `owning_guid`,
> `volume_guid`) are its synthetic join keys, minted by its engine alone
> from natural record fields via the spindle registry — no Go tool emits
> one, computes one, or carries one through. Every capability in this paper
> is an extraction capability; anything that would synthesise, join or
> normalise on the Go side is out of scope by this rule, wherever it
> appears below — with ONE sanctioned join (decision 14): the image's own
> knowledge store. Layer 1 (`gohost`, `gousers`, `gonetwork`) extracts
> what the image says about itself, and the daemon parsers may resolve
> against it — uid to name, the host's zone and identity — fill-only and
> beside the native values. Cross-record correlation and the guids remain
> byakugan's.
>
> **2. The Go toolz are treated as parsers, with plaso-grade provenance.**
> For every record it is easy to say what was parsed and where that file
> existed: the parser that produced it (`Tool` + `RecordType` — the role
> plaso's `Parser` field plays), the file it parsed (`SourceFilename`),
> and the file's place in the evidence (`Origin`: image → volume →
> snapshot/residue → original volume path, §3.3) — the traceability a
> dfVFS path spec gives a plaso event. No tool writes any of it by hand:
> the `pinfo` runtime stamps it from the access layer's manifest, one
> implementation for the whole matrix.

## 1. Goal and non-goals

**Goal.** Parity of *approach*, not of artefact list: single-purpose static Go
parsers for the Linux artefact classes, self-orchestrating under the
environment contract ([03](../framework/03-environment-contract.md)),
hardened by construction ([05](../framework/05-hardening-standard.md)), fed
either loose files or a disk image through a native Go access layer, and
consumed by DX_DFIR and byakugan the same way the Windows tools are.

**Non-goals.**

- No change to the Windows path in this plan's phases. The Windows tools keep
  their `image_export` staging and their VSS handling as they are; switching
  them to `gomount materialise` and to the `pinfo` module are follow-ups this
  plan enables but does not schedule (§10, §11).
- No live-response agent. Like the Windows matrix, everything here parses
  evidence at rest (an image, a mounted root, a staged tree) — offline, no
  network, read-only.
- No generic super-timeline engine. Plaso's breadth is not replicated;
  byakugan's CAR model is the normalisation layer, fed by per-artefact
  records, as it already is for Windows.

## 2. What "replicate what's done for Windows" means

The Windows shape being replicated, and the two log2timeline couplings being
removed:

| Piece | Windows today | Linux today | Linux target |
|---|---|---|---|
| Per-artefact parsers | 12 static Go tools (goevtx, gomft, gore, …) | Plaso parsers (syslog, utmp, cron, …) inside `log2timeline` | 12 static Go tools (§4) |
| Shared runtime | `batch.go`, byte-identical copy in every tool dir | — | the `pinfo` module (§3), imported not copied |
| Extraction from disk images | Plaso `image_export` + filter file in the lane; `gomount materialise` (Windows artefact sets) and `stream`→`--tar` already started as the native replacement | Plaso `log2timeline` over the whole image | `gomount materialise --set linux-core` (§5) — the same native path, second OS |
| Snapshots | VSS via `PLASO_*_VSS=1` (in-lane) or libvshadow on the host | LVM/Btrfs snapshots not traversed at all | `gomount --snap all` over LVM/Btrfs/qcow2 snapshots (§6) |
| Filesystem timeline | gomft over a materialised `$MFT` | Plaso `filestat` + `l2t_filestat` map | `gomount timeline` MACB records per volume and snapshot (§5.4) |
| Filesystem residue | deleted `$MFT` entries ride gomft's parse; USN via l2t | nothing — `lost+found` at best walked as ordinary files | typed residue surface: `lost+found`, orphan inodes, deleted dirents, fs journals (§5.5) |
| CAR mapping | direct maps on parser output (`goevtx.jsonl` input pattern) | `l2t_utmp`, `l2t_utmpx`, `plaso_exec_cron`, `l2t_text` adapters | direct maps on the Linux tools' JSONL (§7.3) |

The Plaso image itself stays in the repository: it remains the Windows lane's
export stage — until that lane completes the switch to `gomount materialise`
that gomount already began (§10) — and a cross-validation reference during
bring-up. What ends here is Linux processing *depending* on it.

## 3. `pinfo` — the shared parser module

### 3.1 Why a module now

The batch runtime is today a 400-line `batch.go` kept byte-identical in
fourteen tool directories by convention — nothing asserts it but the file's
own header comment and review discipline. That
was the right call for retrofitting twelve existing tools; it is the wrong
starting point for twelve new ones, and the Linux set needs strictly more
shared surface: a common record envelope, timestamp normalisation across a
dozen text and binary formats, and snapshot provenance. That shared surface
becomes a real module, named **`pinfo`** — parser infrastructure, and the
run-introspection role plaso's `pinfo` plays for a `.plaso` storage (§3.5).

### 3.2 Shape

`pinfo/` is a directory-level Go module at the repo root,
`github.com/Get-Sybers/GoDFIR-toolz/pinfo`, with no dependency on any parser
and no cgo:

```
pinfo/
├── go.mod
├── batch/      the runtime: env contract, discovery loop, per-item .part
│               commit, skip-unless-FORCE, the summary line, exit 0/1/2/3
│               (a port of today's batch.go, semantics unchanged) — plus
│               the rule-2 provenance stamping: Origin/Snapshot/Residue
│               joined from the stage manifest onto every record (§3.3)
├── record/     the envelope (§3.3), typed-record schema declaration (the
│               names contract.yml and byakugan's field_provenance cite,
│               §4.1) + the JSONL writer with honest-null discipline —
│               JSONL is the ONE output format of the Linux matrix
│               (decision 12)
├── tstamp/     normalisation to ISO 8601 UTC at fixed microsecond
│               precision (one rendering, lexically sortable): RFC3164
│               (year inference from file mtime with rollover walk-back),
│               RFC5424, ISO-8601 inputs, epoch s/ms/µs/ns, journal usec,
│               days-since-epoch (shadow)
├── discover/   walk + content-first magic detection helpers; transparent
│               rotation (.1, .2.gz …) and gzip; deterministic ordering
├── tarstream/  the --tar mode: consume `gomount stream` (one tar entry per
│               file, entry name = volume path) — one implementation instead
│               of today's per-tool copies
├── contract/   --version / --print-contract handling and the contract.yml
│               embed hook, so conform.sh and CI validate one code path
└── report/     read an OUT_DIR tree (summary lines + record files) and
                answer: which tools ran, over which items, how many records,
                over what time range — the plaso-pinfo analogue (§3.5)
```

A tool binds to it the way it binds to `batch.go` today — the `batchTool`
struct becomes `pinfo/batch.Tool` with the same four fields (`name`,
`formats`, `discover`, `process`); `runFrameworkEntry` keeps its name and
behaviour. The environment contract, summary schema, exit table, and argv
pass-through of [03](../framework/03-environment-contract.md)/
[04](../framework/04-self-orchestration.md) are unchanged — the module is a
relocation plus additions, not a redesign.

### 3.3 The record envelope

Every Linux record is the tool's own flat payload plus a common envelope, so
downstream (byakugan maps, Filebeat, the report verb) reads one shape:

```json
{"Tool":"goauditd","ToolVersion":"0.1.0","RecordType":"auditd_event",
 "SourceFilename":"var/log/audit/audit.log.1.gz","SourceModified":"2026-03-02T04:11:09Z",
 "EventTime":"2026-03-01T22:14:02.481000Z","TimeKind":"event",
 "Origin":{"Image":"srv01.E01","Volume":"vg0/root",
           "Path":"/var/log/audit/audit.log.1.gz","Inode":131204},
 "Snapshot":{"Backend":"lvm","ID":"home-snap1","Time":"2026-02-28T00:00:04.000000Z"},
 "...payload fields..."}
```

- `EventTime` is ISO 8601 UTC at fixed microsecond precision
  (`2006-01-02T15:04:05.000000Z`) — one rendering for every timestamp in
  every record, uniform and lexically sortable — always populated when the
  artefact carries a time; `TimeKind` says what the time is (`event`, `written`, `deleted`,
  `install`, …) for artefacts with several.
- `Tool` + `RecordType` is the record's parser chain — the role plaso's
  `Parser` field plays on the rows byakugan keeps (rule 2).
- `Origin` is where the parsed file existed in the evidence: the image, the
  volume (partition index or `vg/lv`), the original volume path and inode —
  the dfVFS-path-spec role. The `pinfo` batch runtime stamps it (rule 2) by
  joining the item's staged path against the access layer's manifest when
  one is present at the input root, or from the stream's tar entry metadata
  in `--tar` mode; for loose evidence it is absent and `SourceFilename` is
  the whole truth. Tool code never computes it.
- `Snapshot` is present only for records that came out of a snapshot (§6.4);
  absent means the live volume.
- `Host` is the imaged host's own identity — hostname, machine id, OS,
  and the timezone that was applied to naive timestamps — stamped by the
  runtime from the Layer-1 knowledge store (decision 14); absent when no
  store was mounted. Image-self-knowledge, never an inference.
- `Residue` (`Kind`, `Detail`) is its sibling for records recovered from
  filesystem residue — `lost+found`, orphan inodes, deleted directory
  entries, journal history (§5.5); absent means an ordinary allocated file.
  Both it and `Snapshot` are resolved by the runtime alongside `Origin`,
  from the same manifest — never by tool code parsing path prefixes.
- Payload fields stay flat and tool-specific, exactly like the Windows tools'
  records (`goprefetch`'s `Executable`/`RunCount`/… pattern). Their design
  is governed by the byakugan-alignment
  rules of §4.1: typed rows, native vocabulary verbatim, honest nulls, and
  the field set each CAR map consumes as the floor.
- The envelope carries **no synthetic identifiers**: record identity is
  natural fields only (§4.1), and byakugan's guids never appear in parser
  output — rule 1.

The Windows tools' record shapes are not migrated by this plan; when they
adopt `pinfo` (§10) they keep their existing fields and gain nothing
mandatory.

### 3.4 Build shape

Today each Go tool builds with its own directory as context and `COPY *.go`.
A shared module changes that one line of shape: **the parser image builds
with the repo root as context** — the precedent anamnesis, plaso, byakugan
and signatures already set — and the Dockerfile copies `pinfo/` beside the
tool:

```dockerfile
COPY pinfo/ /src/pinfo/
COPY godaemonhunter/ /src/godaemonhunter/
WORKDIR /src/godaemonhunter   # go.mod: require …/pinfo v0.0.0 + replace => ../pinfo
```

The `replace ../pinfo` directive keeps the build hermetic and air-gap-clean:
`pinfo` is versioned by the checked-out tree and the release tag, never
fetched. External dependencies keep the existing `go mod download`-from-lockfile
discipline ([07](../framework/07-supply-chain-and-versioning.md)). A
repo-root `go.work` covers local development; Docker builds do not read it.
The summary line gains a `pinfo` version key so a record tree states which
runtime produced it.

### 3.5 The namesake: run and output introspection

`pinfo/report` ships as a tiny `pinfo` binary (same Tier-1 image shape) whose
batch mode walks an output tree — the thing plaso's `pinfo.py` does for a
storage file, done for our on-disk layout: per tool and per item, the record
count, the time span of `EventTime`, the snapshot set seen, and the summary
lines' status/failure roll-up, as one JSONL report. DX_DFIR's verify/gate step
and the operator get one place that answers "what did this evidence produce?"
without opening record files. It reads outputs only; it is not on any parsing
path — and it reports on *runs*, not evidence: it never joins records across
tools, never mints an identity, and is no substitute for byakugan's timeline
or cross-source views (rule 1). With `Origin` on every record (rule 2), the
report also answers plaso-pinfo's other half — what was parsed and where
each file existed, per evidence item.

### 3.6 Rules the module keeps

`pinfo` changes no contract semantics: stdout is still exactly one JSON line;
records still go to files under `OUT_DIR`; exit codes stay `0/1/2/3`
([§3.5](../framework/03-environment-contract.md)); idempotent re-runs and the
`.part`-then-rename commit stay as they are. `conform.sh` gains one check
(tools under `pinfo` must not carry a local `batch.go`) and loses none.

## 4. The Linux parser set

Fourteen tools, one artefact class each, all Tier-1 shape from birth —
and one organising principle (decision 13): **everything is a daemon
parser**. Each tool reads one daemon's records: the core parses what a
daemon *writes* (journald — the relay every other daemon logs through —
auditd, the login machinery, the syslog daemons, the shells, the package
managers, the kernel's own accounting), and the supporting tools parse
what a daemon *is told* (sshd's and PAM's account material, crond's
tabs, systemd's units, the kernel's sysctl and module policy, the
network daemons' profiles, the mount generators' fstab). The set is
capped by that principle: growth happens by deepening the daemons'
record streams, never by adding parsers for things no daemon owns.

The told-side splits once more into **Layer 1 — the knowledge builders**:
`gohost`, `gousers` and `gonetwork` run FIRST and their output trees are
the image's **knowledge store** — who the host is (hostname, machine-id,
timezone, os-release), who the accounts are (uid↔name, groups, keys),
how it talks and mounts. The daemon parsers then run with that store
mounted read-only (`<TOOL>_KNOWLEDGE_DIR`, the `knowledge` mount) and are
**enriched by it** (decision 14): every record carries the `Host` block,
numeric ids gain resolved names beside them (`UID` stays `1000`,
`UIDName` says `alice` — per this image's own passwd), and naive
timestamps are interpreted in the host's zone. The plaso preprocessing
analogue, done by the matrix's own Layer-1 parsers.

The matrix ships as **one structured binary** — `godaemonhunter`
(decisions 15–17): every parser is a package of the godaemonhunter
module, run through the layered one-shot that puts Layer 1 into
`<OUT_DIR>/knowledge` and then runs the daemon parsers with that store
mounted — the whole method in one run, one output tree, one aggregate
summary line. **The parameter is the stream** (decision 17): a byakugan
model word — `authentication`, `user_session`, `process`, `service`,
`flow`, `file`, `module` — scoping the run to the daemon parsers that
feed that model; no arguments is every stream, the default. It is the
multi-tool dispatcher shape of
[§4.3](../framework/04-self-orchestration.md) (the plaso and signatures
precedent), and it is the **only** shipped shape: there are no
standalone per-parser binaries or images, the argv debug modes ride the
dispatcher (`godaemonhunter <subtool> -f …`), and godaemonhunter defines
no record shape of its own — the packages do.

Every tool is `FROM scratch`, static, `USER 2000:2000`, with a
`contract.yml`, batch mode on no arguments, argv/`--tar` debug pass-through, JSONL output (the one record
format, decision 12). Prior-art libraries are **candidates**: each is license-checked and
pinned per [07](../framework/07-supply-chain-and-versioning.md) at
implementation time; where no permissive pure-Go library holds up, the format
is clean-roomed from its documentation — these are documented formats, and
clean-room over an `io.ReaderAt` is how gomount's partition code was built.

| Tool | Daemon | Reads | Artefacts | Phase |
|---|---|---|---|---|
| `gojournal` | systemd-journald — the relay every daemon logs through | writes | systemd journal `*.journal` files | P1 |
| `goauditd` | auditd / kauditd | writes | auditd `audit.log*` | P1 |
| `gowtmp` | the login machinery (login, sshd, systemd-logind) | writes | wtmp/utmp/btmp, lastlog, wtmpdb | P0 (pilot) |
| `gosyslog` | rsyslog / syslog-ng — the fallback relay | writes | syslog-family text logs | P1 |
| `goshell` | the user's shells and REPLs | writes | shell/REPL histories | P1 |
| `gousers` | sshd, PAM and the account machinery | is told | passwd/shadow/group, sudoers, SSH material | P1 |
| `gocron` | crond / anacron / atd | is told | crontabs, cron.d, anacron, at | P1 |
| `gounit` | systemd (pid 1) itself | is told | units/timers + enablement | P1 |
| `gotrash` | the desktop's trash machinery (gvfsd-trash) | writes | XDG Trash | P1 |
| `gohost` | systemd-hostnamed/timedated + the mount generators | is told | host identity + fstab/crypttab volume mapping | P1 |
| `gonetwork` | NetworkManager / systemd-networkd / the resolver / netfilter | is told | network configuration surface | P1 |
| `goctl` | the kernel and the dynamic loader | is told | sysctl, modprobe, ld.so | P1 |
| `gopkg` | dpkg/rpm/pacman/apk and snapd | writes | package logs, state stores, snap/flatpak | P4 |
| `goacct` | the kernel's process accounting | writes | `pacct` | P4 |

(The Windows analogues the earlier revisions tabulated — goevtx for the
record streams, gore's registry surfaces for the told-side — hold
unchanged; the daemon column is the organising principle.)

### 4.1 Records are designed against the byakugan data model

The data models these records align to are byakugan's, at their remote
origin: the CAR object definitions in
[`Byakugan/model/car/objects`](https://github.com/Get-Sybers/Byakugan/tree/main/model/car/objects)
(one YAML per object — `user_session`, `process`, `file`, `authentication`,
… — generated from the pinned MITRE CAR submodule) and the row-identity
registry
[`byakugan/spindle.yml`](https://github.com/Get-Sybers/Byakugan/blob/main/byakugan/spindle.yml).
The operative model version is whatever `BYAKUGAN_REF` pins in
[`byakugan/Dockerfile`](../../byakugan/Dockerfile) — that is the engine the
pipeline runs — so each tool's field floor is written against the pinned
ref and re-checked when the pin advances, exactly like every other engine
coupling in this repository.
byakugan's store consumes typed rows and decides everything else itself:
which values are canonical for a CAR object and which stay native (its
"canonical column or honest null, never a near-miss" rule), how a row's guid
is minted (the spindle registry, from record fields alone), and every
relationship (`cascade_relationships`, `crosssource`, `derive`). The
parsers' whole job is to hand it the native truth, complete. Six rules bind
every Linux tool's record design:

1. **Typed rows.** Every record carries a stable `RecordType` discriminator
   (the role Plaso's `data_type` plays), and known high-value line families
   are typed *by the parser*: `gosyslog` types sshd `Accepted`/`Failed`,
   sudo, su and cron-session lines the way Plaso's SSH plugin types
   `syslog:ssh:login` — because maps select rows with predicates over typed
   fields and never regex raw messages byakugan-side. The raw line always
   rides along in the record.
2. **Native vocabulary, verbatim.** utmp record types stay `6/7/8`, audit
   record types stay `SYSCALL`/`EXECVE`, flags stay flags. No parser
   pre-normalises into CAR vocabulary — the map's canonical-or-null
   judgement only works when it receives the native value to judge (the
   existing utmp map records `login_type` natively precisely because utmp's
   vocabulary is *not* CAR's).
3. **Honest nulls.** A field the artefact does not carry is omitted from
   the JSON object — never written as `-`, `N/A` or an invented value —
   and a real zero (root's uid) is emitted, not blanked.
4. **Identity fields extracted, never minted.** Each tool documents which
   payload fields identify a record — journal (boot id, seqnum), audit
   (`sec.usec:serial`), utmp (pid, terminal, time), a timeline row (inode,
   path, kind, time) — so spindle registry entries (or an external identity
   form) mint row guids from fields alone, as today, with no parser change.
   The parsers only *carry* those natural fields: computing a spindle guid
   is the engine's alone, and no synthetic guid ever appears in parser
   output (rule 1).
5. **Join keys carried, relationships never derived.** PIDs, UIDs,
   terminals, unit names, paths are extracted exactly as the artefact
   states them; session pairing, parentage, cross-source correlation and
   every enrichment stay in byakugan. A parser that starts joining records
   is out of scope by design.
6. **Field floors from the existing maps.** Where a Plaso-based map exists,
   the field set it consumes is the new tool's minimum: `gowtmp` ⊇ the
   `l2t_utmp` set (username, source hostname, ip_address, pid, terminal,
   terminal_identifier, exit_status, login_type); `gosyslog`'s typed ssh
   rows ⊇ the `syslog:ssh:login` set; `gomount timeline` ⊇ the
   `l2t_filestat` set (§5.4). Classes with no existing map (journal,
   auditd, packages) get their field set written together with their first
   map. Each tool's `contract.yml` declares its record fields (name, type,
   one-line semantic); CI holds the golden records to those names, and a
   generated source definition's `field_provenance` cites exactly them —
   so parser output and map input cannot drift silently.

### 4.2 Per-tool briefs

What is parsed and the known format edges:

- **`gojournal`** — the Linux evtx. Binary journal files from
  `var/log/journal/<machine-id>/` (and `run/log/journal` when staged),
  including archived and `~`-suffixed dirty files; entry payloads compressed
  with XZ, LZ4 or ZSTD by header flag (pure-Go decompressors exist for all
  three). One record per entry: `__REALTIME` µs → `EventTime`, monotonic +
  boot ID, and the field set (`MESSAGE`, `PRIORITY`, `_PID`, `_UID`, `_COMM`,
  `_EXE`, `_CMDLINE`, `_SYSTEMD_UNIT`, `SYSLOG_IDENTIFIER`, `_HOSTNAME`, …)
  flattened. Journals are large: the parser streams entries and never buffers
  a file. Prior art: Velociraptor parses journals in pure Go (AGPL — a
  reference that the format is tractable, not code to reuse); the format is
  documented by systemd.
- **`goauditd`** — audit.log text records; records sharing one
  `msg=audit(sec.usec:serial)` are coalesced into one event (SYSCALL + EXECVE
  + PATH + CWD + PROCTITLE …), hex-encoded fields (proctitle, quoted execve
  args) decoded, argv reassembled in order — the execve stream is the CAR
  `process` feed. Candidate: `elastic/go-libaudit` (Apache-2.0) for record
  parsing/coalescing rules.
- **`gowtmp`** — the P0 pilot: small, fixed-record binary formats with
  immediate CAR value. Classic glibc `struct utmp` (384-byte LE records) for
  wtmp/utmp/btmp including rotated `wtmp.1(.gz)`; `lastlog` (292-byte
  per-UID sparse records, UID from offset); the wtmpdb SQLite successor via
  the cgo-free SQLite driver (decision 12's interim-storage driver). Record-size sanity checks guard
  against non-glibc layouts rather than misparsing them.
- **`gosyslog`** — syslog-shaped text logs (the FALLBACK pathway: on a
  systemd host the same events flow through the journal first, and the
  shared `pinfo/families` engine gives one shape from either source) (`syslog`, `messages`, `auth.log`,
  `secure`, `kern.log`, `cron`, `daemon.log`, mail logs, …) plus their
  rotations, gzip included. Timestamp dialects: RFC3164 (no year — inferred
  from file mtime, walking back across New Year), RFC5424, and ISO-8601
  prefixes; continuation lines attach to their event. One record per line:
  time, host, ident, PID, message, source file and line number.
- **`goshell`** — per-user histories under `home/*` and `root`:
  `.bash_history` (with `HISTTIMEFORMAT` `#<epoch>` stamp lines when
  present), `.zsh_history` (extended `: <epoch>:<elapsed>;cmd` format,
  zsh metafied-byte unescaping), fish, plus `.python_history`,
  `.mysql_history`, `.psql_history`, `.lesshst`, `.viminfo` command lines.
  Sequence order is preserved; `EventTime` is set only when the artefact
  really carries one — no invented times.
- **`gousers`** — account and access surface: `etc/passwd`, `etc/shadow`
  (day-counts → dates; locked/empty markers), `etc/group`/`gshadow`,
  `etc/sudoers` + `sudoers.d` (conservative directive/spec parse),
  `etc/ssh/sshd_config` + drop-ins, per-user `authorized_keys` (options
  captured) and `known_hosts`, host key fingerprints. Typed records
  (`Kind: account|sudoers|sshkey|…`) in one output.
- **`gocron`** — `etc/crontab`, `etc/cron.d/*`, run-parts membership of
  `cron.{hourly,daily,weekly,monthly}`, user spools (`var/spool/cron/crontabs`
  Debian-style and `var/spool/cron` RH-style), `etc/anacrontab`, the at spool
  (job environment + payload).
- **`gounit`** — systemd persistence surface: unit files across
  `etc/systemd/system`, `run/systemd/system`, `usr/lib|lib/systemd/system`
  and user units; enablement from `*.wants/`/`*.requires/` symlinks; masked
  units; timers with their `OnCalendar`/`OnBoot*` spec; `Exec*`, `User`,
  `Environment` lines; the vendor-vs-`/etc` override flag that makes a
  dropped-in unit stand out.
- **`gotrash`** — XDG Trash (`.local/share/Trash` and per-volume
  `.Trash-<uid>`): each `info/*.trashinfo` (original path, deletion time)
  joined with its `files/` twin's size — the `$I`/`$R` of Linux.
- **`gohost`** — the anchoring facts: os-release, hostname, machine-id
  (joins gojournal's `MachineID`), timezone (`etc/timezone`, and the TZif
  trailing POSIX rule from a staged `localtime` — the symlink's zone name
  is lost in staging and honestly absent) and locale — the context
  byakugan needs to place naive timestamps and name the host; plus the
  **volume-to-name mapping**, the drive-serial role on Windows: fstab rows
  (`UUID=`/`LABEL=`/`PARTUUID=` specs split out) tie durable volume
  identities to mount points, crypttab rows tie encrypted devices to
  mapper names. Extraction only: applying the zone and joining UUIDs to
  volumes (`Origin.FSUUID`) stay byakugan-side.
- **`gonetwork`** — the network posture as typed records: hosts, resolv,
  nsswitch, TCP wrappers, interface and connection profiles (ifupdown,
  systemd-networkd, NetworkManager `.nmconnection` with id/type/uuid/SSID
  lifted and everything else verbatim — security material included, the
  shadow-crypt rule), and persisted firewall state (iptables-save chains
  and rules; netplan and nftables.conf captured verbatim, 64KiB cap).
- **`goctl`** — the kernel/loader control surface, a persistence and
  anti-forensics classic, each record with its vendor/admin/runtime Scope:
  sysctl parameters (`sysctl.conf` + `sysctl.d`), module policy
  (`modules-load.d`, `etc/modules`, and `modprobe.d` where `install`/
  `remove` values are shell commands and `blacklist` hides modules), and
  the dynamic loader controls — `ld.so.preload` one record per library,
  `ld.so.conf` paths and includes.
- **`gopkg`** — software presence and package events. Inventories: dpkg
  `status`, rpmdb (`var/lib/rpm` and `usr/lib/sysimage/rpm`; BerkeleyDB,
  ndb and SQLite backends — candidate `knqyf263/go-rpmdb`, pure Go), pacman
  `local/*/desc`, apk `installed`, snapd (`state.json` +
  `snaps/*.snap` — squashfs read, §5.1 — for `meta/snap.yaml`), flatpak
  deploy metadata. Events: `dpkg.log*`, `apt/history.log*`, `pacman.log` —
  timestamped install/upgrade/remove records.
- **`goacct`** — BSD process accounting `var/log/account/pacct*` (`acct_v3`
  64-byte records: comm, uid/gid, tty, btime, elapsed/CPU comp_t, exit and
  flags) — execution history with timestamps, the closest native thing to
  prefetch. sysstat `sa` and atop raw files are version-tied binary formats:
  deliberately out of v1 (§11.2).

Windows classes with no Linux analogue (registry, ESE, prefetch, shellbags,
LNK, jump lists) get none; Linux surfaces Windows lacks (journal, auditd,
package managers) are first-class above. Deferred candidates — web-server
access logs, XDG `recently-used.xbel`, netplan/nftables field-level
decoding — are backlog, listed in §11.3 (browser/application data is out
of the method by decision 13).

## 5. Evidence access: gomount grows Linux filesystems

gomount is already the start of native disk extraction on the Windows side:
`materialise` with its artefact-set catalogue and `stream` feeding the
parsers' `--tar` mode exist precisely to displace the Plaso export stage,
and its internal seams are ready for a second OS — `image` (raw/E01 →
`io.ReaderAt`) and `partition` (MBR/GPT) are filesystem-agnostic; only the
`ntfs*` packages are NTFS-specific. The plan continues that line in the same
tool — same verbs, new backends — rather than introducing a second mount tool
(§11.1 records the decision):

### 5.1 Filesystem backends

A small `fsx` interface (open a volume `ReaderAt` → enumerate, stat, open
files; expose all timestamps the filesystem has, owner/mode/inode, link
targets unfollowed, nlink — plus **allocation state** and a per-backend
**residue enumeration** seam, §5.5, designed in from the first backend
rather than bolted on) with backends:

| FS | Detection | Notes | Candidates |
|---|---|---|---|
| ext2/3/4 | magic `0xEF53` at sb+56 | crtime from 256-byte inodes; extents and legacy block maps | `masahiro331/go-ext4-filesystem`, `dsoprea/go-ext4` |
| XFS | `XFSB` at 0 | v5 crtime | `masahiro331/go-xfs-filesystem` |
| Btrfs | magic at 0x10040 | subvolumes = the snapshot backend (§6.1); no production pure-Go reader exists — **the largest single build item**, clean-room | — |
| vfat | boot sector | `/boot/efi`, USB media | `diskfs/go-diskfs` |
| squashfs | `hsqs` | snap packages, live-ISO roots | `CalebQ42/squashfs`, `diskfs/go-diskfs` |

The userspace path stays the default (parse in-process, no privilege, no
`/dev/fuse`), matching the NTFS backend's philosophy: a malformed filesystem
is a Go error, not a kernel fault. The FUSE `mount` verb remains
NTFS-via-ntfs-3g only; Linux filesystems are served userspace-only until a
concrete need says otherwise.

### 5.2 The volume stack

Linux images are rarely partition→filesystem. Between `partition` and `fsx`
sits a container-peeling layer, applied repeatedly until a filesystem is
reached:

```
image (raw | E01 | qcow2 …)
  └─ partition (MBR/GPT — exists today)
       └─ mdraid?  (superblock 1.x; RAID 0/1 assembly)          [P4]
            └─ LUKS?  (detect always; decrypt only with an
                       operator-supplied key/passphrase file)    [open §11.2]
                 └─ LVM2?  (PV label scan → text VG metadata →
                            LV extent maps: linear, striped;
                            snapshot-cow §6.2; thin/tmeta P4)    [P2]
                      └─ filesystem probe → fsx backend
```

Every verb that names a volume today gains `--lv <vg/lv>` addressing beside
`--volume N`. A new **`identify`** verb prints the whole resolved stack — image
format, partitions, RAID/LUKS/LVM findings, per-volume filesystem, OS guess
(`etc/os-release` vs `Windows/System32`), and the snapshot inventory (§6) — as
one JSON document. It is the lane's routing and reporting input, and the
`snaps` listing lives inside it.

### 5.3 Image formats

`image` gains qcow2 (magic `QFI\xfb`, already in the evidence taxonomy;
candidate `lima-vm/go-qcow2reader`, backing chains followed read-only) beside
raw/dd and E01/Ex01. VHD/VHDX/VMDK follow as candidates in the same seam
(Velocidex-ecosystem readers exist) — they serve the VM_files lane and are not
on the Linux critical path.

### 5.4 `materialise --set linux-core` and `timeline`


- **`linux-core`** joins `materialise-sets.yml` (same embedded catalogue, same
  glob + siblings semantics): `var/log/**` (journal, audit, wtmp/btmp,
  lastlog, syslog family, dpkg/apt/pacman logs), `etc/`
  {passwd, shadow, group, gshadow, sudoers + sudoers.d, crontab + cron.*,
  anacrontab, ssh, systemd, os-release, hostname, hosts, fstab,
  ld.so.preload, modules-load.d}, `var/spool/{cron,at}/**`,
  `usr/lib|lib/systemd/system/**`, `lost+found/**` (§5.5),
  `var/lib/{dpkg/status, rpm/**,
  pacman/local/**, snapd/state.json}`, and per-user
  `home/*|root/{.bash_history, .zsh_history, …, .ssh/**,
  .config/systemd/user/**, .local/share/Trash/**}`. SQLite siblings
  (`-wal`, `-shm`) ride the existing sibling rule. materialise gains a
  `--max-file-size` guard (journals can be tens of GB); every skip lands in
  the manifest, never silent. The manifest itself becomes the **origin
  record** of rule 2: one row per staged file carrying the full chain —
  evidence image, volume (partition index or `vg/lv`), the volume's
  **filesystem UUID and label** (the durable identity fstab and crypttab
  rows name — `Origin.FSUUID`/`Origin.Label`, the drive-serial role),
  snapshot or residue identity, the original volume path, inode, size,
  mtime — the join the `pinfo` runtime uses to stamp `Origin` onto every
  parser record (§3.3), the way a dfVFS path spec rides every plaso
  event.
- **`timeline`** emits one record per **(file, timestamp kind)** — the
  `fs:stat` shape the existing CAR file maps consume, so their
  timestamp-kind → file-action logic (create/modify/read; a kind with no
  canonical action, like ctime, honestly stays raw) ports directly. Each row:
  `TimeKind` (birth, modify, access, change — whatever the backend exposes,
  ext4 crtime and Btrfs otime included), `EventTime`, path, size, mode,
  owner uid/gid, inode, nlink, link target, in the envelope shape (§3.3),
  per volume and, under `--snap`, per snapshot. Every row carries the
  entry's **allocation state** (the `InUse` gomft already emits for `$MFT`
  entries), and `--residue` adds the recovered rows of §5.5. `--hash` adds
  the file's own md5/sha1/sha256 — canonical by the filestat precedent: the
  stat'ed file *is* the file. It replaces `filestat`+`l2t_filestat` in the
  Linux path.

### 5.5 Filesystem residue: `lost+found`, orphans, deleted entries, journals

Filesystems leave artefacts behind, and the Windows matrix already uses its
own (gomft emits deleted `$MFT` entries as `InUse: false` rows). The Linux
backends expose theirs through one typed residue surface on `fsx` — each
item a kind, whatever metadata survives, and content where it is still
addressable:

| Kind | What it is | Backend |
|---|---|---|
| `lost_found` | fsck-recovered orphans under `lost+found/` (`#<inode>` names — the original path is gone, the inode metadata and content are not) | ext4, XFS (`xfs_repair`), Btrfs (`check --repair`) |
| `orphan_inode` | unlinked-but-intact inodes: the superblock orphan list and inode-table sweep for in-range inodes with no directory entry | ext4 first |
| `deleted_dirent` | names still readable in directory-block slack — the *filename and parent* of a deleted file, joined to its inode when that survives | ext4, vfat (0xE5 entries) |
| `fs_journal` | metadata history out of the filesystem journal: prior inode versions, dropped dirents, commit sequence — recent deletes and renames with times | ext4/jbd2 (P4); XFS log is backlog (§11.3) |
| `backup_root` | previous metadata-tree generations from superblock backup roots — bounded time-travel *beside* snapshots | Btrfs (listed by `identify`) |

How it flows, consistent with everything else in this plan:

- **`timeline --residue`** emits the recovered entries as ordinary rows —
  allocation state, residue kind and whatever timestamps survive, native
  and verbatim per §4.1 — so byakugan's file maps judge them (a recovered
  delete with a real time is a `file` delete; a nameless orphan without one
  honestly stays metadata/raw). Nothing recovered is ever silently mixed
  in with allocated files: the kind and the `Residue` provenance are on
  every row.
- **`materialise --residue`** (and `stream --residue`) pulls recoverable
  *content* into the stage under `residue/<kind>/<id>/…`, manifest rows
  included — so a deleted-then-recovered `auth.log` or shell history is
  parsed by the same gosyslog/goshell sub-tools as its live sibling, and
  the signatures lane scans recovered bytes it would otherwise never see.
  The parsers stay residue-agnostic exactly as they are snapshot-agnostic.
- **Envelope:** a `Residue` object (`Kind`, `Detail`) parallels `Snapshot`
  in §3.3 — explicit on access-layer rows, stamped onto parser records by
  the `pinfo` runtime from the manifest (rule 2), with the `residue/…`
  staged path as the human-readable trace.
- **Scope line:** this is structure-driven recovery — what the filesystem's
  own metadata still proves. Content carving over unallocated space is a
  different discipline and stays out (backlog, §11.3); the signatures lane
  is the place raw-space scanning already lives.

Phasing: allocation state and the `lost+found`/`orphan_inode`/
`deleted_dirent` kinds land with the ext4 backend in P2; Btrfs
`backup_root` listing joins the snapshot work in P3; jbd2 journal decoding
is P4.

### 5.6 Contract and hardening posture

gomount keeps `entrypoint: argv` (a declared deviation — its verbs are
streaming) and its existing image shape; nothing here adds a shell
dependency. The Linux backends are pure Go in the same binary. `conform.sh`
and the gate apply unchanged. Whether gomount becomes a first-class
`images.yml` entry is already framework open question #3; this plan makes it
load-bearing for a whole OS family, which is an argument the decision record
should note (§11.2).

## 6. Snapshots: passing snaps to the parsers

VSS parity is the point: on Windows every store is processed so pre-wipe and
pre-rollback state is seen. The Linux equivalents are enumerated, read, and
passed through the same verbs, and the parsers stay snapshot-agnostic — they
just see more inputs, with provenance in the path and the envelope.

### 6.1 Btrfs snapshots (P3)

Snapshots are subvolumes. The Btrfs backend enumerates every subvolume from
the root tree with its id, path, parent UUID, read-only flag and `otime`; a
snapshot is just a subvolume the operator (or `--snap all`) selects. Distro
layouts (snapper's `.snapshots/<n>/snapshot`, Timeshift trees, `@`/`@home`)
need no special-casing — they are subvolumes like any other, and `identify`
labels the recognisable conventions.

### 6.2 LVM snapshots (P3 classic, P4 thin)

- **Classic COW snapshots:** the snapshot LV's exception store (chunk-mapped
  COW format) is overlaid on the origin LV read-only — origin plus exceptions
  reconstructs the volume as of snapshot time.
- **Thin snapshots:** thin LVs and their snapshots share the pool's `tmeta`
  device — a superblock and a two-level B-tree mapping virtual to data
  blocks. Same seam, more parsing; lands with the thin-pool work in P4.

### 6.3 VM-format and ZFS snapshots

qcow2 internal snapshots (the header's snapshot table, each with its own P1
table) are listed by `identify` and readable best-effort in P3 — checkpoint
parity for VM evidence. ZFS is the honest gap: no credible pure-Go reader
exists and kernel mounts are off the table, so ZFS pools are **detected and
reported, not read**, with the userspace-OpenZFS (`zdb`-style, declared
Shape-B deviation) option recorded as the path if demand materialises
(§11.2).

### 6.4 The surface and the provenance rule

- `gomount identify` lists snapshots; `stream`, `materialise` and `timeline`
  take `--snap all` (or a comma list of ids) and iterate the base volume plus
  each selected snapshot.
- **Layout:** the base volume's output is unchanged; snapshot output lands
  under `snap/<backend>-<id>/…` (materialise) or with the same prefix on tar
  entry names (stream). `batchItemName` folds the prefix into item names, so
  per-item outputs stay unique and self-describing with no parser changes.
- **Envelope:** `timeline`/`stream --jsonl` records carry the `Snapshot`
  object (§3.3) explicitly; on parser records the `pinfo` runtime stamps
  `Snapshot` (and `Origin`) from the materialise manifest, which records
  `{snapshot: {backend,id,name,time}}` per pulled file (§5.4) — the CAR
  layer can group or diff by snapshot, and the `snap/…` path prefix stays
  as the human-readable trace in `SourceFilename`.
- **Dedup:** `--snap-dedup` (default on for materialise, like Plaso's VSS
  behaviour) skips a snapshot file whose path, size and mtime match the base
  copy; content-hash comparison is opt-in. Everything skipped is in the
  manifest.

### 6.5 The other "snaps"

Ubuntu snap *packages* are also covered, deliberately: `gopkg` inventories
installed snaps from `snapd/state.json` and reads each `*.snap` (squashfs,
§5.1) for its `meta/snap.yaml` — so both readings of "snaps" — snapshots and
snap packages — are in scope, in their right places (§6.1–6.4 and §4·gopkg).

## 7. Pipeline integration

### 7.1 DX_DFIR lane

The `godfir-toolz` lane already tolerates tools that find nothing (a
non-Windows image exits 1 — tolerated by design), which makes the Linux
integration additive:

1. The lane declares **one more export run per image tree**:
   `gomount materialise --set linux-core --snap all` into the same staging
   root the Plaso `image_export` run uses today. On a Windows image it copies
   nothing and that is not an error; on a Linux image `image_export` stages
   nothing. No routing logic — content decides, as everywhere else.
2. `godaemonhunter` joins `dxdfir_godfir_toolz_tools` as **one entry**
   run bare over the staged tree — every stream, the default (decisions
   15–17; a lane wanting less passes a byakugan-model stream word). The
   layering is internal — Layer 1 (`gohost`, `gousers`, `gonetwork`)
   runs first into `<OUT_DIR>/knowledge` and the daemon parsers come out
   enriched (decision 14) — so the lane needs no per-parser ordering or
   knowledge plumbing. `gomount timeline` output is staged beside its
   tree.
3. `identify` output is captured per image as lane telemetry (and is the
   input for eventually skipping the Plaso export on non-Windows images —
   consumer optimisation, not a correctness need).

Loose/staged Linux evidence (a tarred `/var/log`, a triage collection) needs
no lane change at all: the tools content-detect under `INPUT_DIR` from P1.
Evidence-taxonomy changes: none — Linux disk images are already
`disk_images`/`vm_files`; loose artefact routing to the catch-all is
unchanged, and refining it is a consumer follow-up.

### 7.2 Keeping the pipeline green

The Windows path is untouched at every phase; each Linux capability lands
producer-first, consumer-switch-second, mirroring the framework's migration
rules ([§10.4](../framework/10-migration-roadmap.md)). During P2–P5 the Plaso
lane keeps running over the Linux corpus as a cross-check: a diff harness
(conform.sh-style report) compares native output coverage against the
equivalent Plaso parsers per image — utmp events, syslog line counts,
filestat vs `timeline` — and the switch-off of log2timeline for Linux images
is gated on that report, per class.

### 7.3 byakugan

New direct source maps consume the tools' JSONL by `input_pattern`, the way
`goevtx.jsonl` is consumed today — maps live in
[`byakugan/mappings`](https://github.com/Get-Sybers/Byakugan/tree/main/byakugan/mappings)
(today's Linux coverage is `plaso_linux.py`, the log2timeline-shaped module
this work replaces) with their generated definitions in
[`sources/`](https://github.com/Get-Sybers/Byakugan/tree/main/sources). The
CAR-object targets against the
[`model/car/objects`](https://github.com/Get-Sybers/Byakugan/tree/main/model/car/objects)
definitions (byakugan's model decides the final shapes; §4.1 fixes what the
parsers owe them):

| Source | CAR object · action | Standing today |
|---|---|---|
| `gowtmp` | `user_session` login/logout (record types 6/7/8; others stay raw) | replaces `l2t_utmp`/`l2t_utmpx` |
| the `pinfo/families` typed events — from `gojournal` (the primary pathway) and `gosyslog` (fallback) alike | `user_session` / authentication | replaces the `l2t_text` ssh view; one map serves both sources |
| `goauditd` execve events | `process` create/execute — argv, uids, tty carried | new coverage |
| `goacct` | `process` (execution history) | new coverage |
| `gojournal` (typed units/messages) | `user_session`, `process`, service surface | new coverage |
| `gomount timeline` | `file` via `TimeKind` (§5.4) | replaces `l2t_filestat` |
| `gocron` / `gounit` | the scheduled/persistence surface | replaces `plaso_exec_cron` |
| `gopkg` | software inventory (native/other coverage) | new coverage |

The `l2t_utmp`, `l2t_utmpx`, `plaso_exec_cron` and `l2t_text`/`l2t_filestat`
adapters stay for legacy storages and are retired from the *default* Linux
path once the P5 parity report clears their class. Map authoring happens in
the byakugan repository; this plan fixes the interface it can rely on: the
envelope (§3.3), the §4.1 record rules — typed rows, native vocabulary,
declared field names for `field_provenance`, identity fields for the
spindle registry — snapshot provenance available for grouping, and rule-2
`Origin` giving every row its `SourceImage` roll-up identity natively
(today derived from the processed tree's path layout). Each new
source's generated definition cites the Go tool as `extractor` and
`<tool>.jsonl` as its `input_pattern`, exactly as `goevtx` appears today.

## 8. Fixtures and testing

Same regime as the Windows tools — committed fixtures or deterministic
generators, a contract smoke test per tool asserting exit codes, the single
summary line, idempotency and the config-error path — with the fixture
strategy split by what can be generated unprivileged:

- **Go-generated, in-repo:** utmp/wtmp/btmp/lastlog, pacct, trashinfo trees,
  passwd/shadow/sudoers, crontabs, unit trees, histories, audit.log,
  syslog files (plain and gz), SQLite profile fixtures. Each tool's
  `test/fixtures/` carries a generator, not binaries, where possible.
- **Unprivileged filesystem images:** `mkfs.ext4 -d <rootdir>` and
  `mkfs.btrfs --rootdir` populate a file-backed image with no root and no
  mounts; `mksquashfs` likewise; `qemu-img convert` wraps raw → qcow2. A
  fixture-builder script produces small (≤ 8 MB, sparse) images from
  committed root trees. **Residue fixtures** are generated the same
  unprivileged way: `debugfs -w` unlinks and kills entries on the image
  file without mounting it, deterministically manufacturing deleted
  dirents, orphan inodes and `lost+found` cases for the §5.5 tests.
- **Privileged-once goldens:** Btrfs *snapshots*, LVM (classic COW and thin)
  and mdraid layouts cannot be authored offline with stock tools; a
  `test/fixtures/gen-privileged.sh` builds them on a lab host (loop devices +
  device-mapper), and the resulting small images are committed compressed
  with the script as their provenance. Journal fixtures are captured tiny
  real files (`systemd-journal-remote` from a fixed entry set), committed the
  same way.
- **Gate:** `conform.sh` and the CI gate ([06](../framework/06-verification-gate.md))
  apply to every new directory from its first PR; the `pinfo` module carries
  the runtime's unit tests (today's `batch_test.go` relocated) plus
  golden-record tests per tstamp dialect.

## 9. Framework conformance

Nothing in this plan is exempt: the parsers ship inside `godaemonhunter/`,
one Tier-1 directory per
[02](../framework/02-tool-directory-structure.md) (Dockerfile, contract.yml,
README, tests — with a README per parser package under it), hardened per
[05](../framework/05-hardening-standard.md)
(`FROM scratch`, static, uid 2000, `/etc/dfir-hardened`, full label set), and
it enters `images.yml` on landing. There is deliberately **no new Tier-2 image**:
no Python, no shell, no interpreter anywhere in the Linux path — the property
the Windows matrix had to migrate toward is the Linux matrix's starting
condition. The one structural novelty a reviewer will meet is the shared
module and the repo-root build context (§3.4), which conform.sh learns to
check.

## 10. Phases

Each phase ends with the gate green over its fixtures and the DX_DFIR light
corpus run green; no consumer switch precedes its producer piece.

| Phase | Scope | Lands | Proves |
|---|---|---|---|
| **P0** | `pinfo` + pilot | the module (batch port + envelope + tstamp + report), `gowtmp` built on it, conform.sh/module CI, repo-root build shape | the module and build shape work end-to-end on the smallest real parser |
| **P1** | core parsers | `gojournal`, `goauditd`, `gosyslog`, `goshell`, `gousers`, `gocron`, `gounit`, `gotrash`; tools join the lane list | staged/loose Linux evidence parses natively — no Plaso in that path |
| **P2** | image path — **built** (the lane's export run remains) | gomount: `fsx`, ext4/ext2 + XFS v5 + vfat backends, LVM (linear/striped, `--lv`), `identify`, `linux-core` materialise with the origin-record manifest and `--max-file-size`, `timeline` with allocation state and `--hash`; ext4 residue kinds — `lost+found`, orphan inodes, deleted dirents (§5.5), staged by `materialise --residue`; fixtures built with the real mkfs tools; lane adds the native export run (consumer side) | a plain or LVM Linux disk image processes end-to-end with the Plaso image absent, residue included |
| **P3** | snaps | Btrfs backend + subvolume/snapshot enumeration and `backup_root` residue listing, LVM COW snapshots, `--snap all` + provenance + dedup, qcow2 (+ internal-snapshot listing) | snapshot state reaches every parser with provenance — VSS parity |
| **P4** | second wave | `gopkg` (the package managers' own logs and state stores; squashfs/snap/flatpak included), `goacct`; LVM thin, mdraid; ext4/jbd2 journal residue | the OS record stream completed; the hard volume layouts; journal-derived history |
| **P5** | cutover | byakugan direct maps; parity diff harness vs Plaso per class; DX_DFIR retires log2timeline from the default Linux path; `l2t_*` adapters demoted to legacy | the goal state: Linux CAR built entirely from native parser output |

Follow-ups this plan enables but does not schedule: the Windows tools adopt
`pinfo` (mechanical: delete `batch.go`, import, repo-root context — after P1
proves the module); the Windows lane completes the extraction switch gomount
already started — `image_export` retired in favour of `materialise` over the
existing Windows sets, which by then is the proven Linux path; and a VSS
backend behind the same `--snap` surface, giving both OS families one
snapshot story. Each is a one-page decision when its time comes.

## 11. Decisions and open questions

### 11.1 Decided by this plan

| # | Decision |
|---|---|
| 1 | **The rules** (stated in full after the abstract): (1) the Go toolz never replace byakugan and never use its synthetic joining guids — parsers extract everything byakugan needs to do its job, and nothing on the Go side synthesises, joins or normalises; (2) the toolz are treated as parsers with plaso-grade provenance — every record traceable to its parser, its source file, and where that file existed |
| 2 | No log2timeline anywhere in the Linux path; Plaso remains for the Windows export stage and as a bring-up cross-check only |
| 3 | The shared runtime is a real module, `pinfo`, at the repo root; Linux tools are born on it; the byte-identical-`batch.go` rule remains for the Windows tools until their adoption phase |
| 4 | Parser images build with the repo root as context and take `pinfo` via `replace` — hermetic, air-gap-clean, no module fetch |
| 5 | The record envelope (§3.3) with ISO 8601 UTC `EventTime` (fixed microsecond precision) and explicit provenance is mandatory for every Linux tool — and carries no synthetic identifiers; `Origin`/`Snapshot`/`Residue` are stamped by the `pinfo` runtime from the access layer's manifest, never computed by tool code |
| 6 | gomount — already begun as the native extraction path on Windows — gains the Linux backends behind one `fsx` seam; no second mount tool; userspace-only for Linux filesystems |
| 7 | Snapshots are passed by the access layer (`--snap all`), parsers stay snapshot-agnostic, provenance rides paths + envelope + manifest, dedup defaults on |
| 8 | ZFS is detected and reported, not read, until a demand-driven decision (§11.2) |
| 9 | Every new image is Tier-1/`FROM scratch`; no interpreter enters the Linux path |
| 10 | Record design is byakugan-aligned per §4.1 — typed rows, native vocabulary verbatim, honest nulls, identity fields and join keys extracted (never minted), declared field names — and parsers never derive relationships, canonicalise into CAR vocabulary, or enrich: extraction is the parsers' side of the boundary, derivation is byakugan's |
| 11 | Filesystem residue is an access-layer capability behind `fsx` (§5.5): typed kinds, allocation state on every timeline row, `Residue` provenance parallel to `Snapshot`, recovered content re-fed through the same parsers — structure-driven recovery only, never content carving, and never silently mixed with allocated files |
| 12 | Records are **JSONL only** — one JSON object per record; CSV is not an output format anywhere in the Linux path and no `<TOOL>_FORMAT` variable exists. A tool that ever needs interim storage beyond streaming (sorting or aggregation past memory) uses a database format (SQLite via the cgo-free driver) in its `WORK_DIR` scratch — never an interchange text format — and the record files stay the JSONL interface |
| 14 | **Layer 1 and the knowledge store**: `gohost`, `gousers` and `gonetwork` run first and their output trees are the image's knowledge store; the daemon parsers mount it read-only (`<TOOL>_KNOWLEDGE_DIR`) and the `pinfo` runtime + tools enrich from it — the `Host` block on every record, resolved names beside native numeric ids (`UIDName` beside `UID`), the host's zone applied to naive timestamps (the `Host.Timezone` on the record states it). This is the one sanctioned parser-side join, bounded to image-SELF-knowledge, fill-only, never overwriting a native value; correlation, relationships and guids remain byakugan's. Without the store, records are exactly what they were — enrichment absent, never invented |
| 13 | **The method — everything is a daemon parser**: the matrix reads the OS's own record-keeping — the journal, systemd, and the logs the system produces (auditd, the syslog family, login records, the package managers' logs, kernel accounting). Each tool reads one daemon's stream — the core what a daemon writes, the supporting tools what a daemon is told. That core is where depth is added; the told-side tools already built (`gohost`, `gonetwork`, `goctl`, `gousers`, `gocron`, `goshell`, `gotrash`) are its one-pass supporting context and that direction is closed — no further config-surface parsers, and application-data parsing (browser profiles, generic SQLite dumps) is out of the method. The journal is also the **primary pathway**: the typed event families (sshd, sudo, pam, cron) are defined once in `pinfo/families` and recognised wherever that stream surfaces — the journal first, the flat logs as fallback — so even SSH evidence flows through the journal mechanism, one shape from either source |
| 15 | **godaemonhunter — the one structured binary**: the twelve parsers also ship as a single multi-tool image, the [§4.3](../framework/04-self-orchestration.md) dispatcher shape (the plaso/signatures precedent). `godaemonhunter <subtool>` runs one parser's ordinary env-contract batch under its canonical `<SUBTOOL>_*` block; `godaemonhunter hunt` is the layered one-shot — Layer 1 into `<OUT_DIR>/knowledge`, then every daemon parser with the store mounted (decision-14 enrichment), one aggregate JSON summary line with every sub-tool's summary embedded. godaemonhunter defines no record shape of its own — the parser packages do |
| 16 | **The parsers live inside godaemonhunter — nothing ships singular**: the parser packages are subdirectories of the godaemonhunter module (`godaemonhunter/wtmp`, `godaemonhunter/journal`, …), documented by per-package READMEs; the standalone per-parser binaries, images, contracts and Dockerfiles are retired (decision 15's "per-tool images remain the granular units" clause is superseded). The argv debug modes ride the dispatcher (`godaemonhunter <subtool> -f FILE \| -d DIR \| --tar`). Tool names, `<SUBTOOL>_*` env blocks, record shapes and the `Tool` field on every record are unchanged — byakugan sees the same records; only the packaging is one |
| 17 | **The stream is the parameter**: godaemonhunter is called with a stream word, and every accepted word IS a byakugan model (`model/car/objects` at the `BYAKUGAN_REF` pin) that the matrix feeds — `authentication`, `user_session`, `process`, `service`, `flow`, `file`, `module`. A stream scopes the layered run to the Layer-2 daemon parsers feeding that model's maps (§7.3); **the default — no arguments — is every stream**; Layer 1 is never scoped (it is the knowledge store); model words nothing feeds yet (`registry`, `thread`, …) are rejected with the accepted list, and a word enters the vocabulary only when a parser here feeds it. Sub-tool names stay accepted for granular and argv-debug runs, `hunt` for the explicit default, and the aggregate summary names the streams it ran |

### 11.2 Open questions

1. **ZFS.** If needed: userspace OpenZFS (`zdb`-style extraction) as a
   declared Shape-B deviation image, vs. a pure-Go reader (research-grade
   effort), vs. staying report-only. Decide on real corpus demand.
2. **LUKS.** Detection always; decryption only with operator-provided key
   material — the open part is the provisioning UX (a keyfile mount in the
   lane contract vs. out-of-band pre-decryption on the host). LUKS2/argon2 is
   feasible in pure Go; sequencing it is the question.
3. **Journald library.** Adopt a permissively-licensed pure-Go journal reader
   if one holds up at pinning time; otherwise clean-room (the format is
   documented). AGPL implementations are references, never dependencies.
4. **sysstat/atop.** Version-tied binary formats with real forensic value;
   support matrix and effort unclear — revisit after P4's `goacct`.
5. **When the Windows tools adopt `pinfo`** — after P1, in parallel with
   P2–P3, or batched with their eventual materialise switch.
6. **gomount inventory status** (framework open question #3) — this plan
   makes gomount load-bearing for Linux; that weighs toward first-class
   `images.yml` entry and should be settled by P2.

### 11.3 Backlog (recorded, unscheduled)

Web-server access/error logs (`goweb`); XDG `recently-used.xbel` and desktop
artefacts; netplan/nftables field-level decoding (gonetwork captures them
verbatim today); generic/browser SQLite parsing (`gosqlite` — out of
the method by decision 13, revisit only on explicit demand); XFS log decoding as a `fs_journal` residue backend (§5.5 covers
ext4/jbd2); content carving over unallocated space (out of the residue
surface by decision 11 — if it ever lands, it is the signatures lane's
business); VHD/VHDX/VMDK image formats. A persistence-sweep view across
gounit/gocron/gousers outputs (the autoruns analogue) is recorded here so
it is not re-derived, but under rule 1 it is byakugan's side of the
boundary — a derived view over parser records, never a Go tool.
