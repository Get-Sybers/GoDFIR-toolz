# syntax=docker/dockerfile:1
#
# gomount — mount ONE NTFS volume from a disk image as a real, browsable,
# READ-ONLY mount, UNPRIVILEGED.
#
# Pipeline: image (E01/raw, O_RDONLY) -> io.ReaderAt -> pick NTFS volume + read
# BPB -> bounded io.ReaderAt -> single regular file volume.img over FUSE
# (go-fuse, BSD-3) -> ntfs-3g -o ro mounts that regular file (GPL-2.0, a
# SEPARATE PROCESS, never linked). Unprivileged because ntfs-3g mounts a regular
# file (kernel mount type "fuse", not "fuseblk") and gomount runs root-inside-a-
# user-namespace. No CAP_SYS_ADMIN, no loop device, no --privileged.
#
# DELIBERATE DEVIATION from the other GoDFIR tool images: those are FROM scratch
# (one static binary, nothing else). gomount's runtime is debian:trixie-slim
# because the operator mount EXECs the distro ntfs-3g binary and its fusermount3
# helper — permissive-by-separation (exec, never link libntfs-3g). The Go binary
# itself is still static (CGO off).
#
#   docker build -t get-sybers/gomount:latest -f gomount/Dockerfile gomount
#
# Run (rootless podman — the container is already a user namespace, so no host
# relaxation is needed; this is the recommended path):
#   podman run --rm -it --device /dev/fuse \
#     -v "$PWD/evidence:/evidence:ro" \
#     get-sybers/gomount:latest mount --mount-point /mnt/ntfs /evidence/disk.E01
#   # browse from another shell:  podman exec -it <ctr> ls -l /mnt/ntfs
#
# Run (rootful docker — gomount self-bootstraps a user namespace; if the host or
# daemon blocks unprivileged userns creation, add seccomp=unconfined. STILL no
# CAP_SYS_ADMIN and no --privileged):
#   docker run --rm -it --device /dev/fuse \
#     --security-opt seccomp=unconfined --security-opt apparmor=unconfined \
#     -v "$PWD/evidence:/evidence:ro" \
#     get-sybers/gomount:latest mount /evidence/disk.E01
#
# ALWAYS-LATEST: golang:trixie is the latest stable Go toolchain; debian:trixie-
# slim is the latest stable Debian; apt-get update installs the latest patched
# ntfs-3g + fuse3. mkntfs / mkfs.ntfs ship inside the ntfs-3g package on trixie
# (there is no separate ntfsprogs package), so the smoke test's mkntfs is present.

ARG GO_IMAGE=golang:trixie
ARG RUNTIME_IMAGE=debian:trixie-slim

FROM ${GO_IMAGE} AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Static, trimmed binary — no libc, no cgo. gomount only EXECs ntfs-3g at
# runtime; it never links it.
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /gomount .
# Sanity-gate the build: no args must print usage and exit 1 (a panic/crash or
# any other code fails the build here).
RUN /gomount >/dev/null 2>&1; [ $? -eq 1 ]

FROM ${RUNTIME_IMAGE}
# fuse3  -> fusermount3 (the unprivileged FUSE mount helper / fallback)
# ntfs-3g -> ntfs-3g + lowntfs-3g + mkntfs/mkfs.ntfs + libntfs-3g.so (the only
#            shared lib ntfs-3g needs beyond libc, per ldd).
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        fuse3 \
        ntfs-3g \
 && rm -rf /var/lib/apt/lists/*

COPY --from=build /gomount /usr/local/bin/gomount
# The integration mount test travels with the image so it can run in any
# host that provides /dev/fuse + unprivileged user namespaces.
COPY test/mount-test.sh /test/mount-test.sh

# Single non-root run-as uid/gid, matching the get-sybers image role. An
# unprivileged uid is exactly what maps itself to root-inside-a-userns, so the
# non-root user IS the unprivileged-mount path (no CAP_SYS_ADMIN needed). The
# default mount point is pre-created and owned by that uid.
ARG DFIR_UID=2000
ARG DFIR_GID=2000
RUN mkdir -p /mnt/ntfs \
 && chown ${DFIR_UID}:${DFIR_GID} /mnt/ntfs
USER ${DFIR_UID}:${DFIR_GID}

ENTRYPOINT ["/usr/local/bin/gomount"]

LABEL org.opencontainers.image.title="get-sybers/gomount" \
      org.opencontainers.image.description="Mount one NTFS volume from an E01/raw disk image as a real, browsable, read-only mount, unprivileged: gomount decodes the image, selects the NTFS volume, exports it as a single regular file volume.img over FUSE (go-fuse, BSD-3) sized to the exact volume length, and EXECs the distro ntfs-3g (GPL-2.0, separate process, never linked) to mount that regular file read-only. Runs root-inside-a-user-namespace: no CAP_SYS_ADMIN, no loop device, no --privileged. Needs --device /dev/fuse. debian:trixie-slim runtime (operator-mount deviation from the FROM-scratch tool images)." \
      org.opencontainers.image.licenses="Apache-2.0" \
      com.get-sybers.tool="gomount" \
      com.get-sybers.hardened="true" \
      com.get-sybers.runtime="debian-trixie-slim (execs ntfs-3g; never links libntfs-3g)"
