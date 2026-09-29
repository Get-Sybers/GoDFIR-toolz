# a hardened FROM-scratch stand-in: the posture verify inspects (non-root USER +
# the com.get-sybers.hardened label); it is never run
FROM scratch
ARG DFIR_UID=2000
ARG DFIR_GID=2000
COPY payload /payload
LABEL com.get-sybers.hardened="true"
USER ${DFIR_UID}:${DFIR_GID}
ENTRYPOINT ["/payload"]
