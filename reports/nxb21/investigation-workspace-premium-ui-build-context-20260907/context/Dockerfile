ARG BASE_IMAGE=nexusai/localai-forensic:rollback-before-premium-investigation-ui-20260907
FROM ${BASE_IMAGE}

COPY --chmod=0555 nexusai-ui-wrapper /usr/local/bin/nexusai-ui-wrapper
COPY ui-dist/ /opt/nexusai/unified-ui/

ENV NEXUSAI_BACKEND_URL=http://127.0.0.1:8080 \
    NEXUSAI_BASE_ENTRYPOINT=/entrypoint.sh \
    NEXUSAI_UI_LISTEN=:8088 \
    NEXUSAI_UI_ROOT=/opt/nexusai/unified-ui

EXPOSE 8088

LABEL nexusai.activation="investigation-workspace-premium-ui-20260907" \
      nexusai.base-image="sha256:0e47f2d689e796cf231e08b33f23e470999d3a80000ef852d49d8c93eb05827d" \
      nexusai.d-unqualified-source-included="false"

ENTRYPOINT ["/usr/local/bin/nexusai-ui-wrapper"]
CMD ["phi-2"]
