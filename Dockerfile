# +-------------------------------------------------------------------+
# | (C) Copyright IBM Corp. 2025,2026                                 |
# | SPDX-License-Identifier: Apache-2.0.                              |
# +-------------------------------------------------------------------+

ARG BASE_UBI_IMAGE_TAG=9.6
ARG BUILDER_IMAGE
# Latest UBI image with Go 1.25
FROM ${BUILDER_IMAGE:-registry.access.redhat.com/ubi9/go-toolset:1.25.9-1778675823} AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /build
USER root

COPY api api
COPY cmd cmd
COPY internal internal
COPY pkg pkg
COPY go.mod go.mod
COPY go.sum go.sum
COPY vendor vendor

ARG VERSION=""
ARG BUILD_FLAGS=""

ARG GOTOOLCHAIN=local
ENV GOTOOLCHAIN=${GOTOOLCHAIN}

RUN echo "TARGETARCH: ${TARGETARCH}" && \
    echo "TARGETOS: ${TARGETOS}" && \
    echo -n "GOVERSION: " && go env GOVERSION && \
    echo -n "GOTOOLCHAIN: " && go env GOTOOLCHAIN && \
    CGO_ENABLED=1 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" GO111MODULE=on GOTOOLCHAIN="${GOTOOLCHAIN}" \
    CGO_LDFLAGS_ALLOW='-Wl,--unresolved-symbols=ignore-in-object-files' \
    go build -ldflags "-s -w -X main.version=${VERSION}" -tags strictfipsruntime ${COMMAND_BUILD_OPTIONS} \
    -a -o spyre-dra-plugin cmd/spyre-dra-plugin/main.go

RUN dnf --installroot=/tmp/ubi-micro \
    --nodocs --setopt=install_weak_deps=False \
    install -y \
    pciutils libxml2-devel openssl-libs openssl-fips-provider && \
    dnf --installroot=/tmp/ubi-micro \
    clean all

# generate minimal image
FROM registry.access.redhat.com/ubi9/ubi-micro:${BASE_UBI_IMAGE_TAG}

ARG VERSION

LABEL io.k8s.display-name="Spyre Resource Driver for Dynamic Resource Allocation (DRA)"
LABEL name="Spyre Resource Driver for Dynamic Resource Allocation (DRA)"
LABEL vendor="ibm.com"
LABEL version=${VERSION}
LABEL release=$(RELEASE)
LABEL summary="IBM Spyre DRA resource driver for Kubernetes"
LABEL description="See summary"

COPY --from=builder /tmp/ubi-micro/ /
COPY --from=builder /build/spyre-dra-plugin /usr/bin/spyre-dra-plugin
