ARG BASE_IMAGE=nexusai/localai-forensic:rollback-before-premium-investigation-ui-final-20260907
FROM ${BASE_IMAGE}

COPY --chmod=0555 nexusai-ui-wrapper /usr/local/bin/nexusai-ui-wrapper
COPY ui-dist/ /opt/nexusai/unified-ui/

ENV NEXUSAI_BACKEND_URL=http://127.0.0.1:8080 \
    NEXUSAI_BASE_ENTRYPOINT=/entrypoint.sh \
    NEXUSAI_UI_LISTEN=:8088 \
    NEXUSAI_UI_ROOT=/opt/nexusai/unified-ui

EXPOSE 8088

LABEL nexusai.activation="investigation-workspace-premium-ui-final-20260907" \
      nexusai.base-image="sha256:8782f416eaea46bd0ebb914913ae37ae97263966c391c218e789cbe1ee113df7" \
      nexusai.d-unqualified-source-included="false"

ENTRYPOINT ["/usr/local/bin/nexusai-ui-wrapper"]
CMD ["phi-2"]
