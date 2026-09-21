ARG BASE_IMAGE=nexusai/localai-forensic:rollback-before-conversation-ui-final-20260908
FROM ${BASE_IMAGE}

COPY --chmod=0555 nexusai-ui-wrapper /usr/local/bin/nexusai-ui-wrapper
COPY ui-dist/ /opt/nexusai/unified-ui/

ENV NEXUSAI_BACKEND_URL=http://127.0.0.1:8080 \
    NEXUSAI_BASE_ENTRYPOINT=/entrypoint.sh \
    NEXUSAI_UI_LISTEN=:8088 \
    NEXUSAI_UI_ROOT=/opt/nexusai/unified-ui

EXPOSE 8088

LABEL nexusai.activation="investigation-workspace-conversation-ui-final-20260908" \
      nexusai.base-image="sha256:bed2eec364ca8c65151625685f79f5ab7a606a8223af88d1696504751b4a03a1" \
      nexusai.d-unqualified-source-included="false"

ENTRYPOINT ["/usr/local/bin/nexusai-ui-wrapper"]
CMD ["phi-2"]
