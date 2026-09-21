ARG BASE_IMAGE=nexusai/localai-forensic:rollback-before-unified-workspace-ui-20260907
FROM ${BASE_IMAGE}

COPY --chmod=0555 nexusai-ui-wrapper /usr/local/bin/nexusai-ui-wrapper
COPY ui-dist/ /opt/nexusai/unified-ui/

ENV NEXUSAI_BACKEND_URL=http://127.0.0.1:8080 \
    NEXUSAI_BASE_ENTRYPOINT=/entrypoint.sh \
    NEXUSAI_UI_LISTEN=:8088 \
    NEXUSAI_UI_ROOT=/opt/nexusai/unified-ui

EXPOSE 8088

LABEL nexusai.activation="investigation-workspace-unified-ui-20260907" \
      nexusai.base-image="sha256:feb935b42a93c1810394cb270a809189ca240a225253159a47b8daa72a799171" \
      nexusai.d-unqualified-source-included="false"

ENTRYPOINT ["/usr/local/bin/nexusai-ui-wrapper"]
CMD ["phi-2"]
