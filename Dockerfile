FROM gcr.io/distroless/static:nonroot
ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/manager /manager
USER nonroot:nonroot
ENTRYPOINT ["/manager"]
