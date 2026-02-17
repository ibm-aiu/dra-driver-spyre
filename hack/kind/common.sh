#!/usr/bin/env bash

# Copyright 2023 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

if [ -z "$DRIVER_NAME" ]; then
    echo "Error: DRIVER_NAME is not set or is empty"
    exit 1
fi

# Install kind if not already installed

# Set default KIND_VERSION if not already set
KIND_VERSION="${KIND_VERSION:-v0.22.0}"
KIND_BIN="${LOCALBIN:-/usr/local/bin}"/kind

# Check if kind is already installed
if ! command -v kind >/dev/null 2>&1; then
    # Detect OS and architecture
    OS="$(uname | tr '[:upper:]' '[:lower:]')"
    ARCH="$(uname -m)"

    # Map architecture to kind binary naming
    case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH" && exit 1 ;;
    esac

    echo "Installing kind ${KIND_VERSION} for ${OS}-${ARCH}..."
    URL="https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-${OS}-${ARCH}"
    curl -Lo ./kind "$URL"
    chmod +x ./kind
    sudo mv ./kind "$KIND_BIN"
    echo "✅ kind installed at $KIND_BIN"
fi

# The name of the kind cluster to create
: ${KIND_CLUSTER_NAME:="${DRIVER_NAME}-cluster"}

# Container tool, e.g. docker/podman
if [[ -z "${CONTAINER_TOOL}" ]]; then
    if [[ -n "$(which docker)" ]]; then
        echo "Docker found in PATH."
        CONTAINER_TOOL=docker
    elif [[ -n "$(which podman)" ]]; then
        echo "Podman found in PATH."
        CONTAINER_TOOL=podman
    else
        echo "No container tool detected. Please install Docker or Podman."
        return 1
    fi
fi

: ${KIND:="env KIND_EXPERIMENTAL_PROVIDER=$(basename "$CONTAINER_TOOL") kind"}
