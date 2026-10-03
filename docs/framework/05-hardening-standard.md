# 05 — The hardening standard

[← 04 — Self-orchestration](04-self-orchestration.md) · [06 — Verification gate →](06-verification-gate.md)

## 5.1 Posture

GoDFIR-toolz images are distroless: the runtime stage carries no shell, no
package manager, no interpreter, and no busybox. There is nothing to exploit
and nothing to `apt install` into. The rules every image obeys:

1. **Minimal final stage.** `FROM scratch` when the tool is a static binary
   (Shape A); otherwise a `FROM scratch` re-copy of a build-time-hardened minimal
   rootfs (Shape B). Never a full distribution with a shell.
2. **Static binaries by default.** `CGO_ENABLED=0 go build -trimpath
   -ldflags='-s -w'`. A dynamic runtime (glibc plus `.so`) is a documented
   exception with a written justification in the Dockerfile header.
3. **Nonroot, fixed identity.** `USER 2000:2000`; uid 0 renamed and locked.
4. **No shell, no package manager, no interpreter** unless the tool provably
   needs one — and then the image declares it (§5.6) and justifies it.
5. **An SBOM per image** and the required label set (§5.7, §5.8).
6. **Pinned, checksum-verified inputs.** anamnesis verifies the MemProcFS
   release sha256; zeek verifies the package-signing key fingerprint;
   godfir-tool verifies its release sha256. Every upstream input is pinned and
   verified this way ([§7.2](07-supply-chain-and-versioning.md)).
7. **Single purpose, minimal closure.** One tool per image; the dependency
   closure is the tool and what it provably needs.

## 5.2 Shape A — static binary, `FROM scratch` (the default)

Used by goevtx, gomft, gore, and the other in-repo Go tools.

```dockerfile
# syntax=docker/dockerfile:1
ARG GO_VERSION=1.25
FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /<tool> .
RUN /<tool> --version              # build-stage sanity gate

FROM scratch
COPY --from=build /<tool> /<tool>
COPY <<EOF /etc/dfir-hardened
schema=1
tool=<tool>
user=dfir uid=2000 gid=2000
static_binary=true
shell=false python=false pkg_mgr=false
EOF
ARG DFIR_UID=2000
ARG DFIR_GID=2000
USER ${DFIR_UID}:${DFIR_GID}
ENTRYPOINT ["/<tool>"]
LABEL ...   # §5.7
```

`FROM scratch` is the strongest posture the standard offers: there is
*nothing* to remove — no shell, no libc, no package manager — so the hardening
contract holds by construction.

## 5.3 Shape B — build-time-hardened minimal rootfs, re-copied into scratch

Used by anamnesis and by any tool that needs glibc, shared objects, or an
interpreter.

- Builder stage(s) produce the binary and its runtime dependencies.
- A `hardened` stage on a slim base runs the build-time hardener
  `hardening/harden.yml` through a *throwaway* Ansible install, then — in the
  same layer — removes Ansible, Python, pip, apt/dpkg, sudo/su, and every shell
  the tool does not need.
- A final `FROM scratch` does `COPY --from=hardened / /`, so the runtime image
  is a flattened minimal rootfs holding only the tool and its justified
  libraries.

Skeleton:

```dockerfile
# syntax=docker/dockerfile:1
# get-sybers/<tool> — <one factual sentence>.
# Shape B: keeps glibc + <lib>.so because <written justification>. No shell, no python.
FROM <builder-base> AS build
# ... produce /opt/<tool>/ (binary + the justified shared objects)

FROM debian:<release>-slim AS hardened
COPY --from=build /opt/<tool> /opt/<tool>
COPY hardening/harden.yml /hardening/harden.yml
RUN set -eu; \
    # throwaway provisioner
    apt-get update && apt-get install -y --no-install-recommends ansible-core; \
    ansible-playbook -c local -i localhost, /hardening/harden.yml; \
    # harden.yml has renamed uid 0, stripped setuid/setgid, removed sudo/su/apt/dpkg/pip,
    # purged caches, and written /etc/dfir-hardened (§5.5). Now remove the provisioner
    # itself and everything the tool does not need, in this same layer:
    rm -rf /hardening /usr/lib/python3* /usr/bin/python3* /usr/bin/ansible* \
           /bin/sh /bin/dash /bin/bash /usr/bin/sh ...

FROM scratch
COPY --from=hardened / /
ARG DFIR_UID=2000
ARG DFIR_GID=2000
ENV HOME=/tmp XDG_CACHE_HOME=/tmp/.cache
USER ${DFIR_UID}:${DFIR_GID}
ENTRYPOINT ["/opt/<tool>/<tool>"]
LABEL ...   # §5.7
```

Keeping glibc and shared objects (anamnesis: MemProcFS `vmm.so` and
`leechcore.so`; plaso: libyal) or an interpreter (byakugan, plaso: Python) is
the **documented exception**. The Dockerfile header states *why* the
interpreter or libc stays. Everything not justified is removed.

## 5.4 Static-binary defaults and the build-stage sanity gate

- `CGO_ENABLED=0 go build -trimpath -ldflags='-s -w'` — static, stripped,
  reproducible-friendly.
- `GOTOOLCHAIN=local` when the build must stay offline (byakugan builds this
  way).
- The Go base is pinned to the toolchain the module graph needs, through the
  `GO_VERSION` build-arg.
- A **build-stage sanity gate** is mandatory: `RUN /<tool> --version`, or the
  usage-exit check the Go tools use (`RUN /<tool>; [ $? -eq 1 ]`). A crash or
  panic fails the build before anything ships.

## 5.5 What the build-time hardener guarantees

`hardening/harden.yml` is the build-time provisioner. It runs inside the image
against localhost during the build and is never present in the runtime image.
It guarantees:

- the uid-0 account is **renamed** to a locked `ansible`/nologin identity and
  uid-0 login is removed (passwd and shadow edited directly);
- the gid-0 group is renamed;
- privilege-escalation and account-manipulation binaries are removed (sudo, su,
  pkexec, passwd, chsh, useradd, mount, …);
- **every setuid/setgid bit is stripped** filesystem-wide;
- package managers and pip are removed (by glob, not by a hand-maintained
  list);
- caches, docs, man pages, and apt state are purged;
- a `USER <uid>:<gid>` runtime identity is set (fixed `2000:2000`);
- the `/etc/dfir-hardened` self-declaration is written (§5.6).

The Dockerfile that invokes it then removes Ansible itself (and Python and the
shell where unneeded) and squashes to the minimal final stage.

## 5.6 The `/etc/dfir-hardened` self-declaration

Every image carries a small, machine-readable record of the posture it claims
for itself, written by `harden.yml` (Shape B) or by a heredoc `COPY` (Shape A).
CI cross-checks it against the actual filesystem
([§6.4](06-verification-gate.md)); DX_DFIR can read it with `docker export`
without a shell in the image.

```
schema=1
tool=<tool>
user=dfir uid=2000 gid=2000
static_binary=true
shell=false python=false pkg_mgr=false
```

| Field | Meaning |
|---|---|
| `schema` | record format version; `1` |
| `tool` | the tool name; equals `com.get-sybers.tool` |
| `user`, `uid`, `gid` | the runtime identity; always `dfir 2000 2000` |
| `static_binary` | `true` when the entrypoint is a static binary with no libc |
| `shell` | `true` only when the image intentionally keeps a shell (declared deviation) |
| `python` | `true` only when the image intentionally keeps a Python interpreter |
| `pkg_mgr` | always `false` in a released image |

The booleans drive the gate's assertion set: an image that declares
`shell=false` is asserted shell-free, one that declares `python=false` is
asserted Python-free, and the set of "tool-only" images is derived from the
declarations rather than from a hand-maintained name list — so it cannot
drift. One record format replaces the two freeform variants (the scratch
tools' two-line file and `harden.yml`'s `user=… root_renamed=…
pkg_mgr_removed=…` form).

## 5.7 Required OCI and `com.get-sybers.*` labels

Every image carries:

```dockerfile
LABEL org.opencontainers.image.title="get-sybers/<tool>" \
      org.opencontainers.image.description="<one factual sentence>" \
      org.opencontainers.image.source="<upstream URL>" \
      org.opencontainers.image.licenses="<SPDX>" \
      org.opencontainers.image.version="<per-image version>" \
      org.opencontainers.image.revision="<GoDFIR-toolz commit sha>" \
      com.get-sybers.tool="<tool>" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1" \
      com.get-sybers.godfir-release="<GoDFIR-toolz release tag>"
```

`com.get-sybers.hardened=true` is the trust anchor DX_DFIR's guard requires.
`version`, `revision`, `godfir-release`, and `contract` let a consumer identify
*which* build it holds without a registry
([§7.3](07-supply-chain-and-versioning.md)). `com.get-sybers.contract` is the
contract schema version the image's `contract.yml` conforms to.

Images built from an external engine repository (anamnesis, byakugan)
additionally carry `com.get-sybers.engine-ref="<pinned source ref>"`, the
`ref:` from `images.yml` that the build cloned. CI asserts it against the
inventory ([§7.5](07-supply-chain-and-versioning.md)).

## 5.8 SBOM generation

CI generates an SBOM per image (SPDX JSON) with `syft <image> -o spdx-json`,
stores it at `sbom/<tool>.spdx.json`, and attaches it to the release bundle
([§7.4](07-supply-chain-and-versioning.md)). The SBOM is a build-time artifact,
not an afterthought: it is what lets the air-gapped consumer audit the
dependency closure without re-scanning, and it is the input to the CVE gate
([08](08-zero-cve-program.md)).

## 5.9 Declared deviations

Three kinds of deviation are sanctioned, each declared in `/etc/dfir-hardened`
and justified in the Dockerfile header:

| Deviation | Declared as | Current holders |
|---|---|---|
| dynamic runtime (glibc + `.so`) | `static_binary=false` | anamnesis (MemProcFS), plaso (libyal) |
| interpreter retained | `python=true` | plaso, byakugan |
| shell retained | `shell=true` | plaso (wrappers), signatures (YARA scan loop), gomount (fusermount) |

Shell-bearing multi-tool images are accepted as declared `shell=true`
deviations. They remain subject to every other rule — nonroot, no package
manager, no setuid bits, the label set, the contract — and the gate asserts the
declaration matches the filesystem. Whether the signatures scan loop and the
plaso wrappers later move into small static Go runners to reach shell-free is a
roadmap option, not a requirement of the standard.

[← 04 — Self-orchestration](04-self-orchestration.md) · [06 — Verification gate →](06-verification-gate.md)
