FROM scratch
LABEL \
    maintainer="Blake Covarrubias <blake@covarrubi.as>, Robert B Gordon <rbg@openrbg.com>" \
    org.opencontainers.image.description="Advertises records for Kubernetes resources over multicast DNS." \
    org.opencontainers.image.licenses="Apache-2.0" \
    org.opencontainers.image.source="git@github.com:grumpylabs/external-mdns" \
    org.opencontainers.image.title="external-mdns" \
    org.opencontainers.image.url="https://github.com/grumpylabs/external-mdns"
ARG TARGETOS
ARG TARGETARCH

COPY bin/external-mdns-${TARGETOS}_${TARGETARCH} /external-mdns
ENTRYPOINT ["/external-mdns"]
