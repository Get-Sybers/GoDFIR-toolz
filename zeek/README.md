# `get-sybers/zeek` — minimal hardened Zeek LTS

Minimal hardened Zeek LTS for **offline capture parsing**, stripped to the
zeek binary + its scripts; ansible (the build-time hardener), python and
every shell are removed from the final image; zkg/zeekctl stripped. `zeek`
is the pinned ENTRYPOINT.

`zeek-lts` comes from the OpenSUSE `security:zeek` OBS repo, fetched from the
**master server** (`downloadcontent.opensuse.org` — direct content, no
mirrorbrain geo-redirect, so a mid-sync mirror can never serve a stale deb),
with the package version pinned (`ZEEK_VERSION`; empty = repo-current) and
the repo signing key verified against a pinned fingerprint before it is
trusted (`ZEEK_KEY_FPR`, fail-closed).

```sh
docker build -t get-sybers/zeek:latest -f zeek/Dockerfile .
```

Builds with the **repo root as context** so `COPY hardening/harden.yml`
consumes the canonical hardener directly; `get-sybers/*` namespace and
[the hardening contract](../README.md#the-hardening-contract) like every
other image here.
