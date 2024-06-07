# Copyright 2018 The Kubernetes Authors.
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

# If you update this file, please follow
# https://suva.sh/posts/well-documented-makefiles

# Ensure Make is run with bash shell as some syntax below is bash-specific
SHELL:=/usr/bin/env bash

.DEFAULT_GOAL:=help

GO_VERSION ?= 1.22.3
GO := $(shell type -P go)
# Use GOPROXY environment variable if set
GOPROXY := $(shell $(GO) env GOPROXY)
ifeq ($(GOPROXY),)
GOPROXY := https://proxy.golang.org
endif
export GOPROXY

# Active module mode, as we use go modules to manage dependencies
export GO111MODULE=on

# Directories.
# Full directory of where the Makefile resides
ROOT_DIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
TOOLS_DIR := hack/tools
APIS_DIR := api
TEST_DIR := test
BIN_DIR := bin
TOOLS_BIN_DIR :=  $(abspath $(TOOLS_DIR)/$(BIN_DIR))

# Set --output-base for conversion-gen if we are not within GOPATH
ifneq ($(abspath $(ROOT_DIR)),$(shell $(GO) env GOPATH)/src/github.com/metal3-io/cluster-api-provider-metal3)
	CONVERSION_GEN_OUTPUT_BASE := --output-base=$(ROOT_DIR)
endif

# Binaries.
CONTROLLER_GEN := $(TOOLS_BIN_DIR)/controller-gen
GOLANGCI_LINT_BIN := golangci-lint
GOLANGCI_LINT := $(TOOLS_BIN_DIR)/$(GOLANGCI_LINT_BIN)
MOCKGEN := $(TOOLS_BIN_DIR)/mockgen
CONVERSION_GEN := $(TOOLS_BIN_DIR)/conversion-gen
KUBEBUILDER := $(TOOLS_BIN_DIR)/kubebuilder
KUSTOMIZE_BIN := kustomize
KUSTOMIZE := $(TOOLS_BIN_DIR)/$(KUSTOMIZE_BIN)
ENVSUBST_BIN := envsubst
ENVSUBST := $(TOOLS_BIN_DIR)/$(ENVSUBST_BIN)-drone
SETUP_ENVTEST = $(TOOLS_BIN_DIR)/setup-envtest
GINKGO_BIN := ginkgo
GINKGO := $(TOOLS_BIN_DIR)/$(GINKGO_BIN)
GINKGO_PKG := github.com/onsi/ginkgo/v2/ginkgo

# Helper function to get dependency version from go.mod
get_go_version = $(shell $(GO) list -m $1 | awk '{print $$2}')
GINGKO_VER := $(call get_go_version,github.com/onsi/ginkgo/v2)
ENVTEST_K8S_VERSION := 1.30.x

# Define Docker related variables. Releases should modify and double check these vars.
# REGISTRY ?= gcr.io/$(shell gcloud config get-value project)
REGISTRY ?= quay.io/metal3-io
STAGING_REGISTRY := quay.io/metal3-io
PROD_REGISTRY := quay.io/metal3-io
IMAGE_NAME ?= cluster-api-provider-metal3
CONTROLLER_IMG ?= $(REGISTRY)/$(IMAGE_NAME)
BMO_IMAGE_NAME ?= baremetal-operator
BMO_CONTROLLER_IMG ?= $(REGISTRY)/$(BMO_IMAGE_NAME)
TAG ?= v1beta1
BMO_TAG ?= capm3-$(TAG)
ARCH ?= $(shell go env GOARCH)
ALL_ARCH = amd64 arm arm64 ppc64le s390x

# Allow overriding manifest generation destination directory
MANIFEST_ROOT ?= config
CRD_ROOT ?= $(MANIFEST_ROOT)/crd/bases
METAL3_CRD_ROOT ?= $(MANIFEST_ROOT)/crd/metal3
METAL3_BMH_CRS ?= $(ROOT_DIR)/examples/metal3plane/hosts.yaml
WEBHOOK_ROOT ?= $(MANIFEST_ROOT)/webhook
RBAC_ROOT ?= $(MANIFEST_ROOT)/rbac

# Allow overriding the imagePullPolicy
PULL_POLICY ?= IfNotPresent

ENVTEST_OS := linux
ifeq ($(shell uname -s), Darwin)
	ENVTEST_OS := darwin
endif

## --------------------------------------
## Help
## --------------------------------------

help:  # Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[0-9A-Za-z_-]+:.*?##/ { printf "  \033[36m%-50s\033[0m %s\n", $$1, $$2 } /^\$$\([0-9A-Za-z_-]+\):.*?##/ { gsub("_","-", $$1); printf "  \033[36m%-50s\033[0m %s\n", tolower(substr($$1, 3, length($$1)-7)), $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

## --------------------------------------
## Testing
## --------------------------------------
##@ tests:

.PHONY: unit
unit: $(SETUP_ENVTEST) ## Run unit test
	$(shell $(SETUP_ENVTEST) use -p env --os $(ENVTEST_OS) --arch $(ARCH) $(ENVTEST_K8S_VERSION)) && \
	$(GO) test ./controllers/... ./baremetal/... \
		--ginkgo.no-color=$(GINKGO_NOCOLOR) \
		$(GO_TEST_FLAGS) \
		$(GINKGO_TEST_FLAGS) \
		-coverprofile ./cover.out && \
	cd $(APIS_DIR) && \
	$(GO) test ./... \
		$(GO_TEST_FLAGS) \
		-coverprofile ./cover.out

unit-cover: unit
	$(GO) tool cover -func=./api/cover.out
	$(GO) tool cover -func=./cover.out

unit-verbose: ## Run unit test
	GO_TEST_FLAGS=-v GINKGO_TEST_FLAGS=-ginkgo.v $(MAKE) unit

unit-cover-verbose:
	GO_TEST_FLAGS=-v GINKGO_TEST_FLAGS=-ginkgo.v $(MAKE) unit-cover

.PHONY: test
test: lint unit ## Run tests

.PHONY: test-e2e
test-e2e: ## Run e2e tests with capi e2e testing framework
	./scripts/ci-e2e.sh

.PHONY: test-clusterclass-e2e
test-clusterclass-e2e: ## Run e2e tests with capi e2e testing framework
	CLUSTER_TOPOLOGY=true GINKGO_FOCUS=basic ./scripts/ci-e2e.sh

GINKGO_NOCOLOR ?= false
ARTIFACTS ?= $(ROOT_DIR)/_artifacts
E2E_CONF_FILE ?= $(ROOT_DIR)/test/e2e/config/e2e_conf.yaml
E2E_OUT_DIR ?= $(ROOT_DIR)/test/e2e/_out
E2E_CONF_FILE_ENVSUBST ?= $(E2E_OUT_DIR)/$(notdir $(E2E_CONF_FILE))
E2E_CONTAINERS ?= quay.io/metal3-io/cluster-api-provider-metal3 quay.io/metal3-io/baremetal-operator quay.io/metal3-io/ip-address-manager

SKIP_CLEANUP ?= false
EPHEMERAL_TEST ?= false
SKIP_CREATE_MGMT_CLUSTER ?= true

## Processes e2e_conf file
.PHONY: e2e-substitutions
e2e-substitutions: $(ENVSUBST)
	mkdir -p $(E2E_OUT_DIR)
	$(ENVSUBST) < $(E2E_CONF_FILE) > $(E2E_CONF_FILE_ENVSUBST)

## --------------------------------------
## Templates
## --------------------------------------
##@ templates
E2E_TEMPLATES_DIR ?= $(ROOT_DIR)/test/e2e/data/infrastructure-metal3
.PHONY: cluster-templates
cluster-templates: $(KUSTOMIZE) ## Generate cluster templates
	$(KUSTOMIZE) build $(E2E_TEMPLATES_DIR)/cluster-template-ubuntu > $(E2E_OUT_DIR)/cluster-template-ubuntu.yaml
	$(KUSTOMIZE) build $(E2E_TEMPLATES_DIR)/cluster-template-centos > $(E2E_OUT_DIR)/cluster-template-centos.yaml
	$(KUSTOMIZE) build $(E2E_TEMPLATES_DIR)/cluster-template-upgrade-workload > $(E2E_OUT_DIR)/cluster-template-upgrade-workload.yaml
	$(KUSTOMIZE) build $(E2E_TEMPLATES_DIR)/cluster-template-centos-md-remediation > $(E2E_OUT_DIR)/cluster-template-centos-md-remediation.yaml
	$(KUSTOMIZE) build $(E2E_TEMPLATES_DIR)/cluster-template-ubuntu-md-remediation > $(E2E_OUT_DIR)/cluster-template-ubuntu-md-remediation.yaml
	touch $(E2E_OUT_DIR)/clusterclass.yaml

.PHONY: clusterclass-templates
clusterclass-templates: $(KUSTOMIZE) ## Generate cluster templates
	$(KUSTOMIZE) build $(E2E_TEMPLATES_DIR)/clusterclass-template-ubuntu > $(E2E_OUT_DIR)/cluster-template-ubuntu.yaml
	$(KUSTOMIZE) build $(E2E_TEMPLATES_DIR)/clusterclass-template-centos > $(E2E_OUT_DIR)/cluster-template-centos.yaml
	$(KUSTOMIZE) build $(E2E_TEMPLATES_DIR)/clusterclass-template-upgrade-workload > $(E2E_OUT_DIR)/cluster-template-upgrade-workload.yaml
	$(KUSTOMIZE) build $(E2E_TEMPLATES_DIR)/clusterclass > $(E2E_OUT_DIR)/clusterclass.yaml

## --------------------------------------
## E2E Testing
## --------------------------------------

GINKGO_FOCUS ?=
GINKGO_SKIP ?=
GINKGO_TIMEOUT ?= 6h

ifneq ($(strip $(GINKGO_SKIP)),)
_SKIP_ARGS := $(foreach arg,$(strip $(GINKGO_SKIP)),-skip="$(arg)")
endif

.PHONY: e2e-tests
e2e-tests: CONTAINER_RUNTIME?=docker # Env variable can override this default
export CONTAINER_RUNTIME

e2e-tests: $(GINKGO) e2e-substitutions cluster-templates # This target should be called from scripts/ci-e2e.sh
	for image in $(E2E_CONTAINERS); do \
		$(CONTAINER_RUNTIME) pull $$image; \
	done

	$(GINKGO) --timeout=$(GINKGO_TIMEOUT) -v --trace --tags=e2e  \
		--show-node-events --no-color=$(GINKGO_NOCOLOR) \
		--junit-report="junit.e2e_suite.1.xml" \
		--focus="$(GINKGO_FOCUS)" $(_SKIP_ARGS) "$(ROOT_DIR)/$(TEST_DIR)/e2e/" -- \
		-e2e.artifacts-folder="$(ARTIFACTS)" \
		-e2e.config="$(E2E_CONF_FILE_ENVSUBST)" \
		-e2e.skip-resource-cleanup=$(SKIP_CLEANUP) \
		-e2e.trigger-ephemeral-test=$(EPHEMERAL_TEST) \
		-e2e.use-existing-cluster=$(SKIP_CREATE_MGMT_CLUSTER)

	rm $(E2E_CONF_FILE_ENVSUBST)

.PHONY: e2e-clusterclass-tests
e2e-clusterclass-tests: CONTAINER_RUNTIME?=docker # Env variable can override this default
export CONTAINER_RUNTIME 

e2e-clusterclass-tests: $(GINKGO) e2e-substitutions clusterclass-templates # This target should be called from scripts/ci-e2e.sh
	for image in $(E2E_CONTAINERS); do \
		$(CONTAINER_RUNTIME) pull $$image; \
	done

	$(GINKGO) --timeout=$(GINKGO_TIMEOUT) -v --trace --tags=e2e  \
		--show-node-events --no-color=$(GINKGO_NOCOLOR) \
		--junit-report="junit.e2e_suite.1.xml" \
		--focus="$(GINKGO_FOCUS)" $(_SKIP_ARGS) "$(ROOT_DIR)/$(TEST_DIR)/e2e/" -- \
		-e2e.artifacts-folder="$(ARTIFACTS)" \
		-e2e.config="$(E2E_CONF_FILE_ENVSUBST)" \
		-e2e.skip-resource-cleanup=$(SKIP_CLEANUP) \
		-e2e.trigger-ephemeral-test=$(EPHEMERAL_TEST) \
		-e2e.use-existing-cluster=$(SKIP_CREATE_MGMT_CLUSTER)

	rm $(E2E_CONF_FILE_ENVSUBST)

## --------------------------------------
## Build
## --------------------------------------
##@ build:

.PHONY: build
build: binaries build-api build-e2e ## Builds all CAPM3 modules

.PHONY: binaries
binaries: manager ## Builds and installs all binaries

.PHONY: manager
manager: ## Build manager binary.
	$(GO) build -o $(BIN_DIR)/manager .

.PHONY: build-api
build-api: ## Builds api directory.
	cd $(APIS_DIR) && $(GO) build ./...

.PHONY: build-e2e
build-e2e: ## Builds test directory.
	cd $(TEST_DIR) && $(GO) build ./...

## --------------------------------------
## Tooling Binaries
## --------------------------------------
##@ tools:

$(CONTROLLER_GEN): $(TOOLS_DIR)/go.mod # Build controller-gen from tools folder.
	cd $(TOOLS_DIR) && $(GO) build -tags=tools -o $(BIN_DIR)/controller-gen sigs.k8s.io/controller-tools/cmd/controller-gen

$(GOLANGCI_LINT):
	hack/ensure-golangci-lint.sh $(TOOLS_DIR)/$(BIN_DIR)

.PHONY: $(GOLANGCI_LINT_BIN)
$(GOLANGCI_LINT_BIN): $(GOLANGCI_LINT) ## Build a local copy of golangci-lint.

$(MOCKGEN): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR) && $(GO) build -tags=tools -o $(BIN_DIR)/mockgen github.com/golang/mock/mockgen

$(CONVERSION_GEN): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR) && $(GO) build -tags=tools -o $(BIN_DIR)/conversion-gen k8s.io/code-generator/cmd/conversion-gen

$(KUBEBUILDER): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR) && ./install_kubebuilder.sh

$(SETUP_ENVTEST): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR) && \
	$(GO) build -tags=tools -o $(BIN_DIR)/setup-envtest sigs.k8s.io/controller-runtime/tools/setup-envtest

.PHONY: $(GINKGO_BIN)
$(GINKGO_BIN): $(GINKGO) ## Build a local copy of ginkgo.

.PHONY: $(GINKGO)
$(GINKGO):
	GOBIN=$(TOOLS_BIN_DIR) $(GO) install $(GINKGO_PKG)@$(GINGKO_VER)

$(ENVSUBST):
	rm -f $(TOOLS_BIN_DIR)/$(ENVSUBST_BIN)*
	cd $(TOOLS_DIR) && $(GO) build -tags=tools -o $(BIN_DIR)/envsubst-drone github.com/drone/envsubst/cmd/envsubst
	ln -sf envsubst-drone $(TOOLS_BIN_DIR)/$(ENVSUBST_BIN)

.PHONY: $(KUSTOMIZE_BIN)
$(KUSTOMIZE_BIN): $(KUSTOMIZE) ## Build a local copy of kustomize.

.PHONY: $(KUSTOMIZE)
$(KUSTOMIZE): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR) && $(GO) build -tags=tools -o $(BIN_DIR)/$(KUSTOMIZE_BIN) sigs.k8s.io/kustomize/kustomize/v5

.PHONY: $(ENVSUBST_BIN)
$(ENVSUBST_BIN): $(ENVSUBST) ## Build envsubst from tools folder.

## --------------------------------------
## Linting
## --------------------------------------
##@ linters:

.PHONY: lint
lint: $(GOLANGCI_LINT) ## Lint codebase
	$(GOLANGCI_LINT) run -v $(GOLANGCI_LINT_EXTRA_ARGS) --timeout=10m
	cd $(APIS_DIR) && $(GOLANGCI_LINT) run -v $(GOLANGCI_LINT_EXTRA_ARGS) --timeout=10m
	cd $(TEST_DIR) && $(GOLANGCI_LINT) run -v $(GOLANGCI_LINT_EXTRA_ARGS) --timeout=10m

.PHONY: lint-fix
lint-fix: $(GOLANGCI_LINT) ## Lint the codebase and run auto-fixers if supported by the linter
	GOLANGCI_LINT_EXTRA_ARGS=--fix $(MAKE) lint

lint-full: $(GOLANGCI_LINT) ## Run slower linters to detect possible issues
	$(GOLANGCI_LINT) run -v --fast=false --timeout=30m
	cd $(APIS_DIR) && $(GOLANGCI_LINT) run -v --fast=false --timeout=30m
	cd $(TEST_DIR) && $(GOLANGCI_LINT) run -v --fast=false --timeout=30m

# Run manifest validation
.PHONY: manifest-lint
manifest-lint:
	./hack/manifestlint.sh

## --------------------------------------
## Generate
## --------------------------------------
##@ generate:

.PHONY: modules
modules: ## Runs go mod to ensure proper vendoring.
	$(GO) mod tidy
	$(GO) mod verify
	cd $(TOOLS_DIR) && $(GO) mod tidy
	cd $(TOOLS_DIR) && $(GO) mod verify
	cd $(APIS_DIR) && $(GO) mod tidy
	cd $(APIS_DIR) && $(GO) mod verify
	cd $(TEST_DIR) && $(GO) mod tidy
	cd $(TEST_DIR) && $(GO) mod verify

.PHONY: generate
generate: ## Generate code
	$(MAKE) generate-go
	$(MAKE) generate-manifests

.PHONY: generate-go
generate-go: $(CONTROLLER_GEN) $(MOCKGEN) $(CONVERSION_GEN) $(KUBEBUILDER) $(KUSTOMIZE) ## Runs Go related generate targets
	$(GO) generate ./...
	cd $(APIS_DIR) && $(GO) generate ./...

	cd ./api && $(CONTROLLER_GEN) \
		paths=./... \
		object:headerFile=../hack/boilerplate/boilerplate.generatego.txt

	$(MOCKGEN) \
	  -destination=./baremetal/mocks/zz_generated.metal3cluster_manager.go \
	  -source=./baremetal/metal3cluster_manager.go \
		-package=baremetal_mocks \
		-copyright_file=./hack/boilerplate/boilerplate.generatego.txt \
		ClusterManagerInterface

	$(MOCKGEN) \
	  -destination=./baremetal/mocks/zz_generated.metal3machine_manager.go \
	  -source=./baremetal/metal3machine_manager.go \
		-package=baremetal_mocks \
		-copyright_file=./hack/boilerplate/boilerplate.generatego.txt \
		MachineManagerInterface

	$(MOCKGEN) \
	  -destination=./baremetal/mocks/zz_generated.metal3datatemplate_manager.go \
	  -source=./baremetal/metal3datatemplate_manager.go \
		-package=baremetal_mocks \
		-copyright_file=./hack/boilerplate/boilerplate.generatego.txt \
		DataTemplateManagerInterface

	$(MOCKGEN) \
	  -destination=./baremetal/mocks/zz_generated.metal3machinetemplate_manager.go \
	  -source=./baremetal/metal3machinetemplate_manager.go \
		-package=baremetal_mocks \
		-copyright_file=./hack/boilerplate/boilerplate.generatego.txt \
		TemplateManagerInterface

	$(MOCKGEN) \
	  -destination=./baremetal/mocks/zz_generated.metal3data_manager.go \
	  -source=./baremetal/metal3data_manager.go \
		-package=baremetal_mocks \
		-copyright_file=./hack/boilerplate/boilerplate.generatego.txt \
		DataManagerInterface

	$(MOCKGEN) \
	  -destination=./baremetal/mocks/zz_generated.metal3remediation_manager.go \
	  -source=./baremetal/metal3remediation_manager.go \
		-package=baremetal_mocks \
		-copyright_file=./hack/boilerplate/boilerplate.generatego.txt \
		RemediationManagerInterface

	$(MOCKGEN) \
	  -destination=./baremetal/mocks/zz_generated.manager_factory.go \
	  -source=./baremetal/manager_factory.go \
		-package=baremetal_mocks \
		-copyright_file=./hack/boilerplate/boilerplate.generatego.txt \
		ManagerFactoryInterface

.PHONY: generate-manifests
generate-manifests: $(CONTROLLER_GEN) ## Generate manifests e.g. CRD, RBAC etc.
	$(CONTROLLER_GEN) \
		paths=./ \
		paths=./api/... \
		paths=./controllers/... \
		crd:crdVersions=v1 \
		rbac:roleName=manager-role \
		output:crd:dir=$(CRD_ROOT) \
		output:rbac:dir=$(RBAC_ROOT) \
		output:webhook:dir=$(WEBHOOK_ROOT) \
		webhook

.PHONY: generate-examples
generate-examples: $(KUSTOMIZE) clean-examples ## Generate examples configurations to run a cluster.
	./examples/generate.sh

.PHONY: generate-examples-clusterclass
generate-examples-clusterclass: $(KUSTOMIZE) clean-examples ## Generate examples configurations to run a cluster.
	CLUSTER_TOPOLOGY=true ./examples/generate.sh

## --------------------------------------
## Docker
## --------------------------------------
##@ docker buils:

.PHONY: docker-build
docker-build: ## Build the docker image for controller-manager
	docker build --network=host --pull --build-arg ARCH=$(ARCH) . -t $(CONTROLLER_IMG)-$(ARCH):$(TAG)
	MANIFEST_IMG=$(CONTROLLER_IMG)-$(ARCH) MANIFEST_TAG=$(TAG) $(MAKE) set-manifest-image
	$(MAKE) set-manifest-pull-policy

.PHONY: docker-push
docker-push: ## Push the docker image
	docker push $(CONTROLLER_IMG)-$(ARCH):$(TAG)

## --------------------------------------
## Docker — All ARCH
## --------------------------------------

.PHONY: docker-build-all ## Build all the architecture docker images
docker-build-all: $(addprefix docker-build-,$(ALL_ARCH))

docker-build-%:
	$(MAKE) ARCH=$* docker-build

.PHONY: docker-push-all ## Push all the architecture docker images
docker-push-all: $(addprefix docker-push-,$(ALL_ARCH))
	$(MAKE) docker-push-manifest

docker-push-%:
	$(MAKE) ARCH=$* docker-push

.PHONY: docker-push-manifest
docker-push-manifest: ## Push the fat manifest docker image.
	## Minimum docker version 18.06.0 is required for creating and pushing manifest images.
	docker manifest create --amend $(CONTROLLER_IMG):$(TAG) $(shell echo $(ALL_ARCH) | sed -e "s~[^ ]*~$(CONTROLLER_IMG)\-&:$(TAG)~g")
	@for arch in $(ALL_ARCH); do docker manifest annotate --arch $${arch} ${CONTROLLER_IMG}:${TAG} ${CONTROLLER_IMG}-$${arch}:${TAG}; done
	docker manifest push --purge ${CONTROLLER_IMG}:${TAG}
	MANIFEST_IMG=$(CONTROLLER_IMG) MANIFEST_TAG=$(TAG) $(MAKE) set-manifest-image
	$(MAKE) set-manifest-pull-policy

.PHONY: set-manifest-image
set-manifest-image:
	$(info Updating kustomize image patch file for manager resource)
	sed -i'' -e 's@image: .*@image: '"${MANIFEST_IMG}:$(MANIFEST_TAG)"'@' ./config/default/capm3/manager_image_patch.yaml

.PHONY: set-manifest-image-ipam
set-manifest-image-ipam:
	$(info Updating kustomize image patch file for IPAM controller)
	sed -i'' -e 's@image: .*@image: '"${MANIFEST_IMG_IPAM}:$(MANIFEST_TAG_IPAM)"'@' ./config/ipam/image_patch.yaml

.PHONY: set-manifest-pull-policy
set-manifest-pull-policy:
	$(info Updating kustomize pull policy file for manager resource)
	sed -i'' -e 's@imagePullPolicy: .*@imagePullPolicy: '"$(PULL_POLICY)"'@' ./config/default/capm3/manager_pull_policy_patch.yaml
	sed -i'' -e 's@imagePullPolicy: .*@imagePullPolicy: '"$(PULL_POLICY)"'@' ./config/ipam/pull_policy_patch.yaml
## --------------------------------------
## Deploying
## --------------------------------------

# Deploy the BaremetalHost CRDs
deploy-bmo-crd:
	kubectl apply -f examples/metal3crds/metal3.io_baremetalhosts.yaml

# Deploy the BaremetalHost resources (for testing purposes only)
deploy-bmo-crs:
	kubectl apply -f "${METAL3_BMH_CRS}"

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: generate-examples
	kubectl apply -f examples/_out/cert-manager.yaml
	kubectl wait --for=condition=Available --timeout=300s -n cert-manager deployment cert-manager
	kubectl wait --for=condition=Available --timeout=300s -n cert-manager deployment cert-manager-cainjector
	kubectl wait --for=condition=Available --timeout=300s -n cert-manager deployment cert-manager-webhook
	kubectl apply -f examples/_out/provider-components.yaml
	kubectl apply -f examples/_out/metal3crds.yaml

deploy-clusterclass: generate-examples-clusterclass
	kubectl apply -f examples/_out/cert-manager.yaml
	kubectl wait --for=condition=Available --timeout=300s -n cert-manager deployment cert-manager
	kubectl wait --for=condition=Available --timeout=300s -n cert-manager deployment cert-manager-cainjector
	kubectl wait --for=condition=Available --timeout=300s -n cert-manager deployment cert-manager-webhook
	kubectl apply -f examples/_out/provider-components.yaml
	kubectl apply -f examples/_out/metal3crds.yaml

deploy-examples:
	kubectl apply -f ./examples/_out/metal3plane.yaml
	kubectl apply -f ./examples/_out/cluster.yaml
	kubectl apply -f ./examples/_out/machinedeployment.yaml
	kubectl apply -f ./examples/_out/controlplane.yaml

deploy-examples-clusterclass:
	kubectl apply -f ./examples/_out/metal3plane.yaml
	kubectl apply -f ./examples/_out/clusterclass.yaml
	kubectl apply -f ./examples/_out/cluster.yaml

delete-examples:
	kubectl delete -f ./examples/_out/machinedeployment.yaml || true
	kubectl delete -f ./examples/_out/controlplane.yaml || true
	kubectl delete -f ./examples/_out/cluster.yaml || true
	kubectl delete -f ./examples/_out/metal3plane.yaml || true

delete-examples-clusterclass:
	kubectl delete -f ./examples/_out/cluster.yaml || true
	kubectl delete -f ./examples/_out/clusterclass.yaml || true
	kubectl delete -f ./examples/_out/metal3plane.yaml || true

## --------------------------------------
## Release
## --------------------------------------
##@ release:

## latest git tag for the commit, e.g., v1.7.0
RELEASE_TAG ?= $(shell git describe --abbrev=0 2>/dev/null)
ifneq (,$(findstring -,$(RELEASE_TAG)))
    PRE_RELEASE=true
endif
# the previous release tag, e.g., v1.7.0, excluding pre-release tags
PREVIOUS_TAG ?= $(shell git tag -l | grep -E "^v[0-9]+\.[0-9]+\.[0-9]+" | sort -V | grep -B1 $(RELEASE_TAG) | grep -E "^v[0-9]+\.[0-9]+\.[0-9]+$$" | head -n 1 2>/dev/null)
RELEASE_DIR := out
RELEASE_NOTES_DIR := releasenotes

$(RELEASE_DIR):
	mkdir -p $(RELEASE_DIR)/

$(RELEASE_NOTES_DIR):
	mkdir -p $(RELEASE_NOTES_DIR)/

.PHONY: release-manifests
release-manifests: $(KUSTOMIZE) $(RELEASE_DIR) ## Builds the manifests to publish with a release
	$(KUSTOMIZE) build config/default > $(RELEASE_DIR)/infrastructure-components.yaml
	cp metadata.yaml $(RELEASE_DIR)/metadata.yaml
	cp examples/clusterctl-templates/clusterctl-cluster.yaml $(RELEASE_DIR)/cluster-template.yaml
	cp examples/clusterctl-templates/example_variables.rc $(RELEASE_DIR)/example_variables.rc

.PHONY: release-notes
release-notes: $(RELEASE_NOTES_DIR) $(RELEASE_NOTES)
	$(GO) run ./hack/tools/release/notes.go --from=$(PREVIOUS_TAG) > $(RELEASE_NOTES_DIR)/$(RELEASE_TAG).md

.PHONY: release
release:
	@if [ -z "${RELEASE_TAG}" ]; then echo "RELEASE_TAG is not set"; exit 1; fi
	@if ! [ -z "$$(git status --porcelain)" ]; then echo "You have uncommitted changes"; exit 1; fi
	git checkout "${RELEASE_TAG}"
	MANIFEST_IMG=$(CONTROLLER_IMG) MANIFEST_TAG=$(RELEASE_TAG) $(MAKE) set-manifest-image
	$(MAKE) release-manifests
	$(MAKE) release-notes

## --------------------------------------
## Tilt / Kind
## --------------------------------------

.PHONY: kind-create
kind-create: ## create capm3 kind cluster if needed
	./hack/kind_with_registry.sh

.PHONY: tilt-settings
tilt-settings:
	./hack/gen_tilt_settings.sh

.PHONY: tilt-settings-clusterclass
tilt-settings-clusterclass:
	CLUSTER_TOPOLOGY=true ./hack/gen_tilt_settings.sh

.PHONY: tilt-up
tilt-up: $(ENVSUBST) $(KUSTOMIZE) kind-create ## start tilt and build kind cluster if needed
	$(MAKE) deploy-bmo-crd
	tilt up

.PHONY: delete-cluster
delete-cluster: delete-workload-cluster  ## Deletes the example kind cluster "capm3"
	kind delete cluster --name=capm3

.PHONY: kind-reset
kind-reset: ## Destroys the "capm3" kind cluster.
	kind delete cluster --name=capm3 || true

## --------------------------------------
## Cleanup / Verification
## --------------------------------------
##@ clean:

.PHONY: clean
clean: ## Remove all generated files
	$(MAKE) clean-bin
	$(MAKE) clean-temporary
	$(MAKE) clean-e2e

.PHONY: clean-bin
clean-bin: ## Remove all generated binaries
	rm -rf bin
	rm -rf hack/tools/bin

.PHONY: clean-temporary
clean-temporary: ## Remove all temporary files and folders
	rm -f minikube.kubeconfig
	rm -f kubeconfig

.PHONY: clean-release
clean-release: ## Remove the release folder
	rm -rf $(RELEASE_DIR)

.PHONY: clean-examples
clean-examples: ## Remove all the temporary files generated in the examples folder
	rm -rf examples/_out/
	rm -f examples/provider-components/provider-components-*.yaml

WORKING_DIR = /opt/metal3-dev-env
M3_DEV_ENV_PATH ?= $(WORKING_DIR)/metal3-dev-env
clean-e2e:
	$(MAKE) clean -C $(M3_DEV_ENV_PATH)
	rm -rf $(E2E_OUT_DIR)

.PHONY: verify
verify: verify-boilerplate verify-modules

.PHONY: verify-boilerplate
verify-boilerplate:
	./hack/verify-boilerplate.sh

.PHONY: verify-modules
verify-modules: modules
	@if !(git diff --quiet HEAD -- go.sum go.mod hack/tools/go.mod hack/tools/go.sum); then \
		echo "go module files are out of date"; exit 1; \
	fi

##@ helpers:
go-version: ## Print the go version we use to compile our binaries and images
	@echo $(GO_VERSION)
