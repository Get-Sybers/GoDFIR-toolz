# syntax=docker/dockerfile:1
#
# gomount — read ONE NTFS volume from a disk image: stream/materialise/ls it in
# userspace (no privilege, no FUSE), or mount it as a real, browsable, READ-ONLY
# mount, UNPRIVILEGED.
#
# Mount pipeline: image (E01/raw, O_RDONLY) -> io.ReaderAt -> pick NTFS volume +
# read BPB -> bounded io.ReaderAt -> single regular file volume.img over FUSE
# (go-fuse, BSD-3) -> ntfs-3g -o ro mounts that regular file (GPL-2.0, a
# SEPARATE PROCESS, never linked). Unprivileged because ntfs-3g mounts a regular
# file (kernel mount type "fuse", not "fuseblk") and gomount runs root-inside-a-
# user-namespace. No CAP_SYS_ADMIN, no loop device, no --privileged.
#
# DECLARED DEVIATION (docs/framework 05 §5.9): the other GoDFIR tool images are
# FROM scratch around one static binary; gomount's runtime is debian:trixie-slim
# because the mount verb EXECs the distro ntfs-3g binary and its fusermount3
# helper — permissive-by-separation (exec, never link libntfs-3g). The Go binary
# itself is static (CGO off). The image declares shell=true (sh ships with the
# base and the ntfs-3g helpers), python=false, pkg_mgr=false (apt/dpkg removed),
# and every setuid/setgid bit is stripped; the runtime is re-copied into scratch.
# Its interface is argv (contract.yml: entrypoint argv) — streaming verbs.
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
ARG TOOL_VERSION=0.3.0
ARG GODFIR_REVISION=unknown
ARG GODFIR_RELEASE=dev

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

FROM ${RUNTIME_IMAGE} AS runtime
ARG DFIR_UID=2000
ARG DFIR_GID=2000
# fuse3  -> fusermount3 (the unprivileged FUSE mount helper / fallback)
# ntfs-3g -> ntfs-3g + lowntfs-3g + mkntfs/mkfs.ntfs + libntfs-3g.so (the only
#            shared lib ntfs-3g needs beyond libc, per ldd).
# Then the hardening the framework asks of every image: the package manager
# goes (nothing is installable at runtime), sudo/su and the account tools go,
# every setuid/setgid bit is stripped (gomount mounts as root-inside-a-userns,
# so fusermount3 needs no setuid), and the self-declaration is written.
RUN set -eux; \
    apt-get update; \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends fuse3 ntfs-3g; \
    rm -rf /var/lib/apt/lists/* /var/cache/apt /var/lib/dpkg /usr/share/doc /usr/share/man; \
    rm -f /usr/bin/apt* /usr/bin/dpkg* /usr/sbin/apt* /usr/bin/add-apt-repository \
          /usr/bin/sudo /usr/bin/su /bin/su /usr/bin/pkexec /usr/bin/passwd /usr/bin/chsh /usr/bin/chfn \
          /usr/bin/gpasswd /usr/bin/newgrp /usr/sbin/usermod /usr/sbin/useradd /usr/sbin/userdel \
          /usr/sbin/groupmod /usr/sbin/groupadd /usr/sbin/groupdel; \
    find / -xdev -perm /6000 -type f -exec chmod a-s {} +; \
    sed -i 's|^root:\([^:]*\):0:0:[^:]*:\([^:]*\):.*$|ansible:\1:0:0:ansible:\2:/usr/sbin/nologin|' /etc/passwd; \
    sed -i 's|^root:[^:]*:|ansible:!:|' /etc/shadow; \
    sed -i 's|^root:|ansible:|' /etc/group /etc/gshadow; \
    mkdir -p /mnt/ntfs; \
    chown ${DFIR_UID}:${DFIR_GID} /mnt/ntfs; \
    printf 'schema=1\ntool=gomount\nuser=dfir uid=%s gid=%s\nstatic_binary=false\nshell=true python=false pkg_mgr=false\n' \
      "${DFIR_UID}" "${DFIR_GID}" > /etc/dfir-hardened; \
    chmod 0444 /etc/dfir-hardened
COPY --from=build /gomount /usr/local/bin/gomount
COPY --chmod=0644 contract.yml /opt/gomount/contract.yml
# The integration mount test travels with the image so it can run in any
# host that provides /dev/fuse + unprivileged user namespaces.
COPY --chmod=0755 test/mount-test.sh /test/mount-test.sh

FROM scratch
COPY --from=runtime / /
ARG DFIR_UID=2000
ARG DFIR_GID=2000
ARG TOOL_VERSION
ARG GODFIR_REVISION
ARG GODFIR_RELEASE
ENV HOME=/tmp XDG_CACHE_HOME=/tmp/.cache
# Single non-root run-as uid/gid, matching the get-sybers image role. An
# unprivileged uid is exactly what maps itself to root-inside-a-userns, so the
# non-root user IS the unprivileged-mount path (no CAP_SYS_ADMIN needed).
USER ${DFIR_UID}:${DFIR_GID}
ENTRYPOINT ["/usr/local/bin/gomount"]
LABEL org.opencontainers.image.title="get-sybers/gomount" \
      org.opencontainers.image.description="Read one NTFS volume from an E01/raw disk image: stream a tar of every file to stdout, materialise artefact sets into a directory, or ls/cat/stat/tree/browse it in userspace with no privilege; or mount it read-only over FUSE + the distro ntfs-3g (a separate process, never linked) root-inside-a-user-namespace: no CAP_SYS_ADMIN, no loop device, no --privileged. debian:trixie-slim runtime (declared deviation: shell kept for the ntfs-3g helpers; apt/dpkg removed, setuid stripped), runs as uid 2000. argv-driven (streaming verbs)." \
      org.opencontainers.image.source="https://github.com/Get-Sybers/GoDFIR-toolz" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.version="${TOOL_VERSION}" \
      org.opencontainers.image.revision="${GODFIR_REVISION}" \
      com.get-sybers.tool="gomount" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1" \
      com.get-sybers.godfir-release="${GODFIR_RELEASE}" \
      com.get-sybers.runtime="debian-trixie-slim (execs ntfs-3g; never links libntfs-3g)"
