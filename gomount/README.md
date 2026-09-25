# `get-sybers/gomount` — read a disk image's volumes, read-only, unprivileged

## Backends

gomount reads the volumes of a disk image in-process, and can additionally
FUSE-mount an NTFS volume.

The **userspace backend** parses filesystems in-process and mounts nothing:
no FUSE, no kernel driver, no privilege. NTFS is read with
[go-ntfs](https://www.velocidex.com/golang/go-ntfs); the Linux filesystems —
**ext2/3/4, XFS v5 and vfat** — with gomount's own clean-room [`fsx`](fsx)
backends (docs/linux §5.1), and every verb resolves ONE volume stack first
(docs/linux §5.2): partitions, **LVM2 volume groups** (linear and striped
LVs, addressed as `--lv vg/lv`), or a bare whole-disk filesystem. It serves
the volume's contents straight from the parser through read verbs — `ls`,
`cat`, `stat`, `tree`, and `browse` for an operator, `stream` and
`materialise` for tools, `identify` for the lane's routing document, and
`timeline` for the fs:stat rows (allocation state on every row; `--residue`
adds the recovered rows of §5.5, `--hash` content digests). A malformed
filesystem surfaces as a Go error rather than a kernel fault, and the
backend runs anywhere a Go binary runs, including containers with no
`/dev/fuse`.

```
gomount ls     <image> [path]      list a directory (default: the volume root)
gomount cat    <image> <path>      write a file's bytes to stdout
gomount stat   <image> <path>      print one entry's metadata
gomount tree   <image> [path]      list a subtree
gomount browse <image>             navigate the volume interactively
gomount identify <image>           print the resolved volume stack as JSON
gomount timeline <image>           one JSONL row per (file, timestamp kind)
gomount stream [--jsonl] <image>   walk the whole filesystem for tools
gomount materialise --out DIR [--set NAME]... [--select GLOB]... [--siblings] [--manifest] <image>
```

`materialise` copies targeted artefacts out of the volume into a real directory,
so a downstream tool consumes them from a plain `-d <dir>`. `--set` names a
built-in artefact set (`registry-core`, `amcache`, `shimcache`, `ntuser`,
`usrclass`, `srum`, `sum`, `timeline`); `--select` adds an ad-hoc volume-path
glob. Each file lands at `<out>/<volume-path>` at mode `0400`. `--siblings`
(default `true`) also copies each artefact's named siblings — a hive's
`.LOG1`/`.LOG2`, a SQLite `-wal`/`-shm` — from the same directory. `--manifest`
writes `<out>/materialise.jsonl` when `--manifest` is given — each row an
**origin record** (docs/linux §5.4): `path/size/mtime` plus the image, the
volume's place in the stack (`p1` | `vg/lv` | `disk`), the filesystem UUID
and label, the inode/mftid, a `staged` path when it differs, a `skip`
reason for files held back by `--max-file-size`, and the residue
kind/detail on `--residue` rows.
The artefact sets are defined in `materialise-sets.yml`, embedded at build time.
A selector that matches nothing copies nothing and is not an error.

The **direct backend** is the `mount` subcommand: it exports the volume as one
regular file over FUSE and execs the distro `ntfs-3g -o ro` onto that file,
producing a real, browsable, read-only mount at a mountpoint. It needs `/dev/fuse`
and unprivileged user namespaces; the rest of this document describes it.

Standalone Go binary that presents a single NTFS volume from a disk image as a
real, browsable, read-only mount without any elevated privilege. It opens the
image `O_RDONLY`, decodes it (raw/dd/img directly, or E01/Ex01 through the
pure-Go [go-ewf](https://github.com/Velocidex/go-ewf) reader), parses the MBR or
GPT partition table clean-room to select the NTFS volume (a partitionless
superfloppy is the whole image), reads the NTFS boot sector via
[go-ntfs](https://www.velocidex.com/golang/go-ntfs) for the exact volume byte
length, and exports that volume as one regular file `volume.img` over FUSE
([go-fuse](https://github.com/hanwen/go-fuse)). `volume.img` reports the exact
volume size in `Getattr`, serves reads straight from the bounded `io.ReaderAt`,
and rejects every write with `EROFS`.

The distro `ntfs-3g` binary then mounts that regular file `-o ro`. `ntfs-3g`
runs as a separate process; gomount execs it and never links `libntfs-3g`, so
gomount stays permissively licensed. Because the backing object is a regular
file rather than a block or loop device, the kernel mount type is `fuse`
(`FS_USERNS_MOUNT`), not `fuseblk`. gomount runs root-inside-a-user-namespace —
it re-execs itself with `CLONE_NEWUSER|CLONE_NEWNS` (the pure-Go equivalent of
`unshare -U -m -r`), or honours an existing user namespace from rootless podman.
No `CAP_SYS_ADMIN`, no loop device, no `--privileged`; the mount needs only
`/dev/fuse` and a kernel that permits unprivileged user namespaces.

The runtime image is `debian:trixie-slim` with `fuse3` and `ntfs-3g`, a
declared deviation from the `FROM scratch` sibling tool images because the
operator mount execs the distro `ntfs-3g` and its `fusermount3` helper. The Go
binary itself is static (CGO off). The image is hardened the same way as the
rest of the matrix: the package manager, sudo/su and the account tools are
removed, every setuid/setgid bit is stripped, uid 0 is renamed and locked, and
it runs as uid 2000; `sh` ships with the base and is declared
(`/etc/dfir-hardened`: `shell=true python=false pkg_mgr=false`).

```sh
docker build -t get-sybers/gomount:latest -f gomount/Dockerfile gomount

# rootless podman (the container is already a user namespace):
podman run --rm -it --device /dev/fuse -v "$PWD/evidence:/evidence:ro" \
  get-sybers/gomount:latest mount --mount-point /mnt/ntfs /evidence/disk.E01

# rootful docker (gomount self-bootstraps the user namespace):
docker run --rm -it --device /dev/fuse \
  --security-opt seccomp=unconfined --security-opt apparmor=unconfined \
  -v "$PWD/evidence:/evidence:ro" \
  get-sybers/gomount:latest mount /evidence/disk.E01
```

```
gomount mount  [--read-only] [--mount-point PATH] [--volume N] [--work PATH] \
               [--no-self-unshare] [--foreground] <image>
gomount umount [--mount-point PATH] [--work PATH]
```

`--volume` is 1-based; `0` auto-selects the largest NTFS volume.
`--no-self-unshare` assumes the caller already established the user namespace.

## Contract

gomount is driven by argv, not by an environment batch loop
([`contract.yml`](contract.yml): `entrypoint: argv`, a declared deviation —
its verbs are streaming and interactive). It reads no `GOMOUNT_*` variables.

## Input

The disk image named in argv, mounted read-only (`/evidence` by convention);
the `mount` verb additionally needs `--device /dev/fuse`.

## Output

- `stream`: a tar archive on stdout, one regular-file entry per volume file (entry name = the file's volume path), or one JSON object per file with `--jsonl`.
- `materialise`: `<out>/<volume-path>` at mode `0400` per pulled file, `residue/<kind>/<id>/<volume-path>` with `--residue`, plus `<out>/materialise.jsonl` with `--manifest`.
- `mount`: a read-only NTFS mount at `--mount-point` held in the foreground until `umount`.
- `ls`/`cat`/`stat`/`tree`/`browse`: the listing or bytes on stdout.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | usage or fatal error (unknown verb, missing image, unreadable volume, mount failure) |
| 2 | unused — declared because the framework's uniform table requires it; gomount never exits 2 |

## Run

```sh
docker run --rm --network none --read-only --cap-drop ALL --security-opt no-new-privileges \
  -v "$PWD/evidence:/evidence:ro" \
  get-sybers/gomount:latest stream --filter '*.pf' /evidence/disk.E01 | goprefetch --tar
```

`test/contract_test.sh` builds the image and asserts the label set, the
self-declaration against the filesystem, and the argv modes (usage on no
arguments, a non-zero exit with nothing on stdout for a missing image);
`test/mount-test.sh` and `test/userspace-test.sh` exercise the verbs on a
host that provides `/dev/fuse` and `mkntfs`.
