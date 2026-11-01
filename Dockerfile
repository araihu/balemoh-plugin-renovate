FROM ghcr.io/renovatebot/renovate@sha256:d3aa7ee1726eb31bf0b8b006f07bcde28747a820e450cf7f4010c45913f5257f
ARG TARGETARCH
USER root
COPY dist/package/ /opt/image-upgrades/package/
COPY dist/collector-${TARGETARCH} /usr/local/bin/image-upgrades-collector
COPY dist/snapshot-server-${TARGETARCH} /usr/local/bin/image-upgrades-server
COPY dist/request-controller-${TARGETARCH} /usr/local/bin/image-upgrades-controller
USER 12021
ENTRYPOINT []
CMD ["/usr/local/bin/image-upgrades-collector"]
