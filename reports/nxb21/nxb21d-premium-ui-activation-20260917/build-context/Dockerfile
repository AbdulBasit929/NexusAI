ARG BASE_IMAGE=nexusai/localai-forensic:rollback-before-nxb21d-premium-ui-20260917
FROM ${BASE_IMAGE}

COPY --chmod=0555 nexusai-ui-wrapper /usr/local/bin/nexusai-ui-wrapper
COPY ui-dist/ /opt/nexusai/unified-ui/

ENV NEXUSAI_BACKEND_URL=http://127.0.0.1:8080 \
    NEXUSAI_BASE_ENTRYPOINT=/entrypoint.sh \
    NEXUSAI_UI_LISTEN=:8088 \
    NEXUSAI_UI_ROOT=/opt/nexusai/unified-ui

EXPOSE 8088

LABEL nexusai.activation="nxb21d-premium-ui-20260917" \
      nexusai.base-image="sha256:91620e4c932764c46931dd4eecb47f9f72408e2387da3189f8fcd4ab4d7506be" \
      nexusai.ui-source-digest="cc104c23d83ffaa0a52a1786594cb9e1845d5cfcf8811ca47b1e526aef004dc6" \
      nexusai.d-unqualified-source-included="false"

ENTRYPOINT ["/usr/local/bin/nexusai-ui-wrapper"]
CMD ["phi-2"]
