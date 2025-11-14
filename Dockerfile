FROM gcr.io/distroless/static:nonroot
ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/manager /manager
USER 65532:65532
ENTRYPOINT ["/manager"]
