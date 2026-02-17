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

# Usage: CONTAINER_TOOL=$(DOCKER) DRIVER_NAME=$(DRIVER_NAME) DRIVER_IMAGE=$(IMAGE) create-cluster.sh

# A reference to the current directory where this script is located
CURRENT_DIR="$(cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)"

set -ex
set -o pipefail

if [ -z "$DRIVER_IMAGE" ]; then
    echo "Error: DRIVER_IMAGE is not set or is empty"
    exit 1
fi

source "${CURRENT_DIR}/common.sh"

# The kubernetes tag to build the kind cluster from
# From ${KIND_K8S_REPO}/tags
# KIND_K8S_REPO:="https://github.com/kubernetes/kubernetes.git"
KIND_K8S_TAG="v1.33.0"

# The name of the kind image to build / run
KIND_IMAGE="kindest/node:${KIND_K8S_TAG}"

# The path to kind's cluster configuration file
KIND_CLUSTER_CONFIG_PATH="${CURRENT_DIR}/kind-cluster-config.yaml"

${KIND} create cluster \
	--name "${KIND_CLUSTER_NAME}" \
	--image "${KIND_IMAGE}" \
	--config "${KIND_CLUSTER_CONFIG_PATH}" \
	--wait 2m

# Work around kind not loading image with podman
${CONTAINER_TOOL} pull ${DRIVER_IMAGE}
IMAGE_ARCHIVE=driver_image.tar
${CONTAINER_TOOL} save -o "${IMAGE_ARCHIVE}" "${DRIVER_IMAGE}" && \
${KIND} load image-archive \
	--name "${KIND_CLUSTER_NAME}" \
	"${IMAGE_ARCHIVE}"
rm "${IMAGE_ARCHIVE}"

set +x
printf '\033[0;32m'
echo "Cluster creation complete: ${KIND_CLUSTER_NAME}"
printf '\033[0m'
