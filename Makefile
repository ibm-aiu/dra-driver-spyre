# +-------------------------------------------------------------------+
# | (C) Copyright IBM Corp. 2025,2026 .                               |
# | SPDX-License-Identifier: Apache-2.0.                              |
# +-------------------------------------------------------------------+

MAKEFILE_PATH		:= $(abspath $(lastword $(MAKEFILE_LIST)))
REPO_ROOT 			:= $(abspath $(patsubst %/,%,$(dir $(MAKEFILE_PATH))))

# Go variables
GOLANG_VERSION		?= $(shell cd $(REPO_ROOT) && go list -f {{.GoVersion}} -m)
GOTOOLCHAIN			?= go1.24.13

# Image variables
## Ensure REGISTRY is updated to your registry before pushing an image.
REGISTRY			?= docker.io/spyre-operator
VERSION				?= $(shell cat $(REPO_ROOT)/VERSION)
DOCKER				?= $(shell command -v podman 2> /dev/null || echo docker)
DOCKERFILE			= $(REPO_ROOT)/Dockerfile
DOCKER_BUILD_OPTS	?= --progress=plain
DRIVER_NAME 		?= dra-driver-spyre
BUILDER_IMAGE		?= registry.access.redhat.com/ubi9/go-toolset:1.24.6-1758501173
MODULE				:= github.com/ibm-aiu/$(DRIVER_NAME)
IMAGE_NAME			=  $(REGISTRY)/$(DRIVER_NAME)
IMAGE_TAG			?= $(VERSION)
IMAGE				?= $(IMAGE_NAME):$(IMAGE_TAG)

# DRA variables
VENDOR				:= ibm.com
APIS				:= spyre/v1alpha1

CODECOV_PERCENT		?= 45

# End to end test configuration variables
E2E_KUBECONFIG			?= ${HOME}/.kube/config
export E2E_KUBECONFIG

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	$(MKDIR) -p $(LOCALBIN)

PATH := $(LOCALBIN):$(PATH)
export PATH

## Tool Binaries
MKDIR			?= mkdir
PYTHON          ?= python3
PIP             ?= pip3
CONTROLLER_GEN	?= $(LOCALBIN)/controller-gen
ENVTEST			?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT	?= $(LOCALBIN)/golangci-lint
GOVULCHECK		?= $(LOCALBIN)/govulncheck
GINKGO			?= $(LOCALBIN)/ginkgo
YQ				?= $(LOCALBIN)/yq
HELM			?= $(LOCALBIN)/helm
LOGCHECK		?= $(LOCALBIN)/logcheck

## Tool Versions
CONTROLLER_TOOLS_VERSION 	?= v0.15.0
ENVTEST_K8S_VERSION			= 1.33
GINKGO_VERSION 				?= v2.28.1
GOLANGCI_LINT_VERSION 		?= 1.64.8
YQ_VERSION 					?= v4.29.2
HELM_VERSION				?= v4.0.0
LOGCHECK_VERSION 			= 0.7.0

# detect-secrets
DETECT_SECRETS_GIT ?= "https://github.com/ibm/detect-secrets.git@master\#egg=detect-secrets"

DOCKER_GO_BUILD_FLAGS ?=

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: version
version: ## Display image version
	@echo "Image version: $(VERSION)"

.PHONY: echo-version
echo-version: ## Print (echo) the current version
	$(info $(VERSION))
	@echo > /dev/null

##@ Development tools
.PHONY: ginkgo
ginkgo: $(GINKGO) ## Download and install ginkgo
$(GINKGO):$(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download and install setup-envtest
$(ENVTEST):$(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@v0.0.0-20240624150636-162a113134de

GOLANGCI_LINT_INSTALL_SCRIPT ?= 'https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh'
.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	test -s $(GOLANGCI_LINT) || { curl -sSfL $(GOLANGCI_LINT_INSTALL_SCRIPT) | bash -s -- -b $(LOCALBIN)  v$(GOLANGCI_LINT_VERSION); }

.PHONY: logcheck-tool
logcheck-tool: $(LOGCHECK) ## Download log check tool
$(LOGCHECK): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/logtools/logcheck@v$(LOGCHECK_VERSION)

.PHONY: govulncheck
govulncheck: $(GOVULCHECK) ## Download govulncheck tool if necessary
$(GOVULCHECK): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install golang.org/x/vuln/cmd/govulncheck@latest

.PHONY: controller-gen
controller-gen: $(LOCALBIN) $(CONTROLLER_GEN) ## Download controller-gen if necessary
$(CONTROLLER_GEN): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: yq
yq: $(YQ) ## Download yq locally if necessary.
$(YQ): $(LOCALBIN)
	test -s $(YQ) || GOBIN=$(LOCALBIN) go install github.com/mikefarah/yq/v4@$(YQ_VERSION)

.PHONY: helm
helm: $(LOCALBIN)
	test -s $(HELM) || curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-4 | HELM_INSTALL_DIR=$(LOCALBIN) bash -s -- -v $(HELM_VERSION)

.PHONY: venv
venv: ## Setup and activate venv
	$(PYTHON) -m venv venv

##@ Test targets

COVERAGE_FILE := coverage.out
.PHONY: test
test: ginkgo envtest vendor checks ## Run unit tests
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GINKGO) run --cover -coverpkg=./... --coverprofile=$(COVERAGE_FILE) -race -v ./test/unit-test/...
	go tool cover -func $(COVERAGE_FILE)
	go tool cover -html $(COVERAGE_FILE) -o coverage-report.html
	@percentage=$$(go tool cover -func=$(COVERAGE_FILE) | grep ^total | awk '{print $$3}' | tr -d '%'); \
		if (( $$(echo "$$percentage < $(CODECOV_PERCENT)" | bc -l) )); then \
			echo "----------"; \
			echo "Total test coverage ($${percentage}%) is less than the coverage threshold ($(CODECOV_PERCENT)%)."; \
			exit 1; \
		else \
			echo "Total test coverage ($${percentage}%) is more than the coverage threshold ($(CODECOV_PERCENT)%)."; \
		fi

##@ E2E Test targets

.PHONY: create-cluster
create-cluster: ## Create Kind cluster
	LOCALBIN=$(LOCALBIN) CONTAINER_TOOL=$(DOCKER) DRIVER_NAME=$(DRIVER_NAME) DRIVER_IMAGE=$(IMAGE) $(REPO_ROOT)/hack/kind/create-cluster.sh

.PHONY: load-driver
load-driver: ## Load local driver image to Kind cluster
	LOCALBIN=$(LOCALBIN) CONTAINER_TOOL=$(DOCKER) DRIVER_NAME=$(DRIVER_NAME) DRIVER_IMAGE=$(IMAGE) $(REPO_ROOT)/hack/kind/load-driver-image-into-kind.sh

.PHONY: deploy-driver
deploy-driver: helm ## Deploy driver to Kind cluster
	$(HELM) upgrade -i \
	--create-namespace \
	--namespace dra-driver-spyre \
	--set image.pullPolicy=IfNotPresent \
	--set image.repository=$(IMAGE_NAME) \
	--set image.tag="$(VERSION)" \
	dra-driver-spyre \
	${REPO_ROOT}/deployments/helm/dra-driver-spyre

.PHONY: delete-cluster
delete-cluster: ## Delete Kind cluster
	LOCALBIN=$(LOCALBIN) CONTAINER_TOOL=$(DOCKER) DRIVER_NAME=$(DRIVER_NAME) $(REPO_ROOT)/hack/kind/delete-cluster.sh

.PHONY: e2e-test
e2e-test: ginkgo ## Run e2e test on the cluster pointed to in the current KUBECONFIG
	$(info E2E_KUBECONFIG is set to $(E2E_KUBECONFIG))
	$(GINKGO) run -race -v ./test/e2e-test/...

##@ Development Targets

.PHONY: fmt
fmt: ## Apply go fmt to the codebase
	go list -f '{{.Dir}}' $(MODULE)/... \
		| xargs gofmt -s -l -w

.PHONY: assert-fmt
assert-fmt: ## Ensure that the code is properly formatted
	go list -f '{{.Dir}}' $(MODULE)/... \
		| xargs gofmt -s -l > fmt.out
	@if [ -s fmt.out ]; then \
		echo "\nERROR: The following files are not formatted:\n"; \
		cat fmt.out; \
		rm fmt.out; \
		exit 1; \
	else \
		rm fmt.out; \
	fi

.PHONY: lint
lint: golangci-lint vendor ## Run golangci-lint against code.
	$(GOLANGCI_LINT) run --sort-results --config $(REPO_ROOT)/.golangci.yaml --go $(GOLANG_VERSION)

.PHONY: lint-fix
lint-fix: golangci-lint vendor ## Run golangci-lint against code.
	$(GOLANGCI_LINT) run --fix --config $(REPO_ROOT)/.golangci.yaml --go $(GOLANG_VERSION)

.PHONY: vulcheck
vulcheck: govulncheck ## Scan for golang vulnerabilities
	$(GOVULCHECK) -show verbose	 ./...

.PHONY: vet
vet: ## Run go vet tool
	go vet -mod vendor $(MODULE)/...

.PHONY: logcheck
logcheck: ## Ensure that all log calls support contextual logging.
	$(LOGCHECK) -check-contextual -check-deprecations ./...

.PHONY: checks
checks: fmt vet lint

CMD_TARGET = $(LOCALBIN)/spyre-dra-plugin
build: vendor $(LOCALBIN) $(CMD_TARGET) ## build binary locally (./bin/spyre-dra-plugin)
$(CMD_TARGET):
	CGO_LDFLAGS_ALLOW='-Wl,--unresolved-symbols=ignore-in-object-files' \
	go build -ldflags "-s -w -X main.version=$(VERSION)" $(COMMAND_BUILD_OPTIONS) -a -o $(CMD_TARGET) cmd/spyre-dra-plugin/main.go

.PHONY: tidy
tidy: ## Apply go tidy
	go mod tidy

.PHONY: vendor
vendor: tidy ## Run go mod vendor command
	go mod vendor

.PHONY: generate ## Generate code
generate: generate-deepcopy

.PHONY: generate-deepcopy ## Generate deepcopy functions
generate-deepcopy: $(CONTROLLER_GEN) vendor
	for api in $(APIS); do \
		rm -f $(CURDIR)/api/$(VENDOR)/resource/$${api}/zz_generated.deepcopy.go; \
		$(CONTROLLER_GEN) \
			object:headerFile=$(CURDIR)/hack/boilerplate.go.txt,year=$(shell date +"%Y") \
			paths=$(CURDIR)/api/$(VENDOR)/resource/$${api}/ \
			output:object:dir=$(CURDIR)/api/$(VENDOR)/resource/$${api}; \
	done

.PHONY: clean
clean: ## Clean-up intermediate artifacts
	-rm -rf vendor
	-rm -rf $(LOCALBIN)

.PHONY: propagate-version
propagate-version: yq ## Propagate version to configuration files
	$(YQ) -i ".image.tag=\"$(VERSION)\"" ${REPO_ROOT}/deployments/helm/dra-driver-spyre/values.yaml

##@ Image operations

.PHONY: docker-build
docker-build: vendor ## Build spyre DRA driver image for build host architecture
	$(DOCKER) build $(DOCKER_BUILD_OPTS) --pull \
	--tag $(IMAGE) \
	--build-arg VERSION="$(VERSION)" \
	--build-arg BUILDER_IMAGE="$(BUILDER_IMAGE)" \
	--build-arg BUILD_FLAGS="$(DOCKER_GO_BUILD_FLAGS)" \
	--file $(DOCKERFILE) $(CURDIR)

.PHONY: docker-push
docker-push: ## Push spyre DRA driver image for the build host architecture.
	$(DOCKER) push $(IMAGE)

.PHONY: docker-build-push
docker-build-push: docker-build docker-push ## Build and push the docker image for the build host

.PHONY: detect-secrets-install
detect-secrets-install: venv ## Install detect-secret tool
	. venv/bin/activate; $(PIP) install "git+$(DETECT_SECRETS_GIT)"

.PHONY: secrets-scan
secrets-scan: venv detect-secrets-install ## Scan secrets and create secret-baseline for repo
	. venv/bin/activate; detect-secrets scan --exclude-files go.sum --update .secrets.baseline

.PHONY: secrets-audit
secrets-audit: venv detect-secrets-install ## Audit secrets
	. venv/bin/activate; detect-secrets audit .secrets.baseline

# helper target for viewing the value of makefile variables.
print-%  : ;@echo $* = $($*)
