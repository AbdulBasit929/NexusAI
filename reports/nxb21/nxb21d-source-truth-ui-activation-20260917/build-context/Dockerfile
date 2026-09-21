ARG BASE_IMAGE=nexusai/localai-forensic:rollback-before-nxb21d-source-truth-ui-20260917
FROM ${BASE_IMAGE}

COPY --chmod=0555 nexusai-ui-wrapper /usr/local/bin/nexusai-ui-wrapper
COPY ui-dist/ /opt/nexusai/unified-ui/

ENV NEXUSAI_BACKEND_URL=http://127.0.0.1:8080 \
    NEXUSAI_BASE_ENTRYPOINT=/entrypoint.sh \
    NEXUSAI_UI_LISTEN=:8088 \
    NEXUSAI_UI_ROOT=/opt/nexusai/unified-ui

EXPOSE 8088

LABEL nexusai.activation="nxb21d-source-truth-ui-20260917" \
      nexusai.base-image="sha256:327754b7fe958c9d3f33f4ad2188f3c29cac7d9576f775b8eaca4c60b3b10006" \
      nexusai.ui-source-digest="c82530503b60b7190a2ac6627c83faacccca822598eaa1ca4cf46d4c7eafd4df" \
      nexusai.d-unqualified-source-included="false"

ENTRYPOINT ["/usr/local/bin/nexusai-ui-wrapper"]
CMD ["phi-2"]
