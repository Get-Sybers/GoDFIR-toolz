# syntax=docker/dockerfile:1
FROM scratch
ARG DFIR_UID=2000
ARG DFIR_GID=2000
ARG TESTPIN=""
ARG GODFIR_REVISION=""
ARG GODFIR_RELEASE=""
COPY payload /payload
COPY <<D /etc/dfir-hardened
schema=1
tool=standin
shell=false python=false pkg_mgr=false
D
LABEL com.get-sybers.hardened="true" testpin="${TESTPIN}" rel="${GODFIR_RELEASE}"
USER ${DFIR_UID}:${DFIR_GID}
ENTRYPOINT ["/payload"]
