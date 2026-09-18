# `get-sybers/gomount` — read one NTFS volume, read-only, unprivileged

## Backends

gomount reads one NTFS volume from a disk image two ways.

The **userspace backend** parses the NTFS filesystem in-process with
[go-ntfs](https://www.velocidex.com/golang/go-ntfs) and mounts nothing: no FUSE,
no kernel driver, no `ntfs-3g`, no privilege. It serves the volume's contents
straight from the parser through read verbs — `ls`, `cat`, `stat`, `tree`, and
`browse` for an operator, and `stream` for tools. A malformed filesystem surfaces
as a Go error rather than a kernel fault, and the backend runs anywhere a Go
binary runs, including containers with no `/dev/fuse`.

```
gomount ls     <image> [path]      list a directory (default: the volume root)
gomount cat    <image> <path>      write a file's bytes to stdout
gomount stat   <image> <path>      print one entry's metadata
gomount tree   <image> [path]      list a subtree
gomount browse <image>             navigate the volume interactively
gomount stream [--jsonl] <image>   walk the whole filesystem for tools
gomount materialise --out DIR [--set NAME]... [--select GLOB]... <image>
```

`materialise` copies targeted artefacts out of the volume into a real directory,
so a downstream tool consumes them from a plain `-d <dir>`. `--set` names a
built-in artefact set (`registry-core`, `amcache`, `shimcache`, `ntuser`,
`usrclass`, `srum`, `sum`, `timeline`); `--select` adds an ad-hoc volume-path
glob. Each file lands at `<out>/<volume-path>` at mode `0400`. `--siblings`
(default `true`) also copies each artefact's named siblings — a hive's
`.LOG1`/`.LOG2`, a SQLite `-wal`/`-shm` — from the same directory. `--manifest`
writes `<out>/materialise.jsonl` with `{path,size,mtime,mftid}` per pulled file.
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
deviation from the `FROM scratch` sibling tool images because the operator mount
execs the distro `ntfs-3g` and its `fusermount3` helper. The Go binary itself is
static (CGO off).

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
