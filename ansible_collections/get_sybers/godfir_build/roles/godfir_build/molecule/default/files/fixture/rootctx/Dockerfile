# syntax=docker/dockerfile:1
FROM scratch
ARG DFIR_UID=2000
ARG DFIR_GID=2000
ARG GODFIR_REVISION=""
ARG GODFIR_RELEASE=""
COPY rootctx-payload /payload
COPY <<D /etc/dfir-hardened
schema=1
tool=rootctx
shell=false python=false pkg_mgr=false
D
LABEL com.get-sybers.hardened="true"
USER ${DFIR_UID}:${DFIR_GID}
ENTRYPOINT ["/payload"]
