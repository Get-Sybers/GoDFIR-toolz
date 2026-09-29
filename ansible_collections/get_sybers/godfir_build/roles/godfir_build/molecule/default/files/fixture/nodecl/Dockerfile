# syntax=docker/dockerfile:1
FROM scratch
ARG DFIR_UID=2000
ARG DFIR_GID=2000
ARG GODFIR_REVISION=""
ARG GODFIR_RELEASE=""
COPY payload /payload
LABEL com.get-sybers.hardened="true"
USER ${DFIR_UID}:${DFIR_GID}
ENTRYPOINT ["/payload"]
