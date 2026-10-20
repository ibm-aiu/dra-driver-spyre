# +-------------------------------------------------------------------+
# | (C) Copyright IBM Corp. 2025,2026                                 |
# | SPDX-License-Identifier: Apache-2.0.                              |
# +-------------------------------------------------------------------+

# TEST-ONLY image of the spyre health service mock used by the e2e tests.
# Build from the repository root (the vendor directory must be up to date):
#   make docker-build-health-mock

ARG BASE_UBI_IMAGE_TAG=9.6
ARG BUILDER_IMAGE
FROM ${BUILDER_IMAGE:-registry.access.redhat.com/ubi9/go-toolset:1.26.7} AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /build
USER root

COPY go.mod go.mod
COPY go.sum go.sum
COPY vendor vendor
COPY test/healthmock test/healthmock

ARG GOTOOLCHAIN=local
ENV GOTOOLCHAIN=${GOTOOLCHAIN}

RUN CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" \
    go build -mod=vendor -ldflags "-s -w" \
    -o spyre-health-mock ./test/healthmock/cmd/spyre-health-mock

FROM registry.access.redhat.com/ubi9/ubi-micro:${BASE_UBI_IMAGE_TAG}

LABEL name="Spyre health service mock (test only)"
LABEL vendor="ibm.com"
LABEL summary="Test-only mock of the Spyre health service. Do not deploy outside of test clusters."

COPY --from=builder /build/spyre-health-mock /usr/bin/spyre-health-mock

USER 1001
ENTRYPOINT ["/usr/bin/spyre-health-mock"]
CMD ["serve"]
