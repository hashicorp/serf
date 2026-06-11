# Copyright IBM Corp. 2013, 2026
# SPDX-License-Identifier: MPL-2.0

FROM golang:1.26 AS devbuild

# Disable CGO to make sure we build static binaries
ENV CGO_ENABLED=0

# Escape the GOPATH
WORKDIR /build
COPY . ./
RUN make pkg/linux_amd64/serf

FROM alpine:3.24 AS dev
ARG PRODUCT_VERSION
ARG PRODUCT_REVISION

COPY --from=devbuild /build/pkg/linux_amd64/serf /bin/

LABEL org.opencontainers.image.title="serf" \
      org.opencontainers.image.description="Serf is a decentralized solution for service discovery and orchestration" \
      org.opencontainers.image.url="https://github.com/hashicorp/serf" \
      org.opencontainers.image.documentation="https://github.com/hashicorp/serf/blob/master/docs/intro/index.html.markdown" \
      org.opencontainers.image.source="https://github.com/hashicorp/serf" \
      org.opencontainers.image.version=${PRODUCT_VERSION} \
      org.opencontainers.image.revision=${PRODUCT_REVISION} \
      org.opencontainers.image.vendor="HashiCorp" \
      org.opencontainers.image.licenses="MPL-2.0"

ENTRYPOINT ["/bin/serf"]
CMD ["help"]
