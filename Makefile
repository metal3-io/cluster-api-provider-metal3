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

# Use GOPROXY environment variable if set
GOPROXY := $(shell go env GOPROXY)
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
TOOLS_BIN_DIR := $(TOOLS_DIR)/bin
BIN_DIR := bin

# Set --output-base for conversion-gen if we are not within GOPATH
ifneq ($(abspath $(ROOT_DIR)),$(shell go env GOPATH)/src/github.com/metal3-io/cluster-api-provider-metal3)
	CONVERSION_GEN_OUTPUT_BASE := --output-base=$(ROOT_DIR)
endif

# Binaries.
CLUSTERCTL := $(BIN_DIR)/clusterctl
CONTROLLER_GEN := $(TOOLS_BIN_DIR)/controller-gen
GOLANGCI_LINT := $(TOOLS_BIN_DIR)/golangci-lint
MOCKGEN := $(TOOLS_BIN_DIR)/mockgen
CONVERSION_GEN := $(TOOLS_BIN_DIR)/conversion-gen
KUBEBUILDER := $(TOOLS_BIN_DIR)/kubebuilder
KUSTOMIZE := $(TOOLS_BIN_DIR)/kustomize
ENVSUBST_BIN := envsubst
ENVSUBST := $(TOOLS_BIN_DIR)/$(ENVSUBST_BIN)-drone
SETUP_ENVTEST = $(TOOLS_BIN_DIR)/setup-envtest
GINKGO := "$(ROOT_DIR)/$(TOOLS_BIN_DIR)/ginkgo"

# Helper function to get dependency version from go.mod
get_go_version = $(shell go list -m $1 | awk '{print $$2}')
GINGKO_VER := $(call get_go_version,github.com/onsi/ginkgo/v2)
ENVTEST_K8S_VERSION := 1.25.x

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
ARCH ?= amd64
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
ARCH ?= amd64

## --------------------------------------
## Help
## --------------------------------------

help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

## --------------------------------------
## Testing
## --------------------------------------

.PHONY: unit
unit: $(SETUP_ENVTEST) ## Run unit test
	$(shell $(SETUP_ENVTEST) use -p env --os $(ENVTEST_OS) --arch $(ARCH) $(ENVTEST_K8S_VERSION)) && \
	go test ./controllers/... ./baremetal/... \
		--ginkgo.no-color=$(GINKGO_NOCOLOR) \
		$(GO_TEST_FLAGS) \
		$(GINKGO_TEST_FLAGS) \
		-coverprofile ./cover.out && \
	cd $(APIS_DIR) && \
	go test ./... \
		$(GO_TEST_FLAGS) \
		-coverprofile ./cover.out

unit-cover: unit
	go tool cover -func=./api/cover.out
	go tool cover -func=./cover.out

unit-verbose: ## Run unit test
	GO_TEST_FLAGS=-v GINKGO_TEST_FLAGS=-ginkgo.v $(MAKE) unit

unit-cover-verbose:
	GO_TEST_FLAGS=-v GINKGO_TEST_FLAGS=-ginkgo.v $(MAKE) unit-cover

.PHONY: test
test: fmt lint unit ## Run tests

.PHONY: test-e2e
test-e2e: ## Run e2e tests with capi e2e testing framework
	./scripts/ci-e2e.sh

GINKGO_NOCOLOR ?= false
ARTIFACTS ?= $(ROOT_DIR)/_artifacts
E2E_CONF_FILE ?= $(ROOT_DIR)/test/e2e/config/e2e_conf.yaml
E2E_OUT_DIR ?= $(ROOT_DIR)/test/e2e/_out
E2E_CONF_FILE_ENVSUBST ?= $(E2E_OUT_DIR)/$(notdir $(E2E_CONF_FILE))
E2E_CONTAINERS ?= quay.io/metal3-io/cluster-api-provider-metal3 quay.io/metal3-io/baremetal-operator quay.io/metal3-io/ip-address-manager

SKIP_CLEANUP ?= false
KEEP_TEST_ENV ?= false
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
E2E_TEMPLATES_DIR ?= $(ROOT_DIR)/test/e2e/data/infrastructure-metal3
.PHONY: cluster-templates
cluster-templates: $(KUSTOMIZE) ## Generate cluster templates
	$(KUSTOMIZE) build $(E2E_TEMPLATES_DIR)/cluster-template-ubuntu > $(E2E_OUT_DIR)/cluster-template-ubuntu.yaml
	$(KUSTOMIZE) build $(E2E_TEMPLATES_DIR)/cluster-template-centos > $(E2E_OUT_DIR)/cluster-template-centos.yaml
	$(KUSTOMIZE) build $(E2E_TEMPLATES_DIR)/cluster-template-upgrade-workload > $(E2E_OUT_DIR)/cluster-template-upgrade-workload.yaml

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
e2e-tests: CONTAINER_RUNTIME?=docker ## Env variable can override this default
export CONTAINER_RUNTIME
e2e-tests: $(GINKGO) e2e-substitutions cluster-templates ## This target should be called from scripts/ci-e2e.sh
	for image in $(E2E_CONTAINERS); do \
		$(CONTAINER_RUNTIME) pull $$image; \
	done

	$(GINKGO) --timeout=$(GINKGO_TIMEOUT) -v --trace --tags=e2e  \
		--show-node-events --no-color=$(GINKGO_NOCOLOR) \
		--fail-fast="$(KEEP_TEST_ENV)" \
		--junit-report="junit.e2e_suite.1.xml" \
		--focus="$(GINKGO_FOCUS)" $(_SKIP_ARGS) "$(ROOT_DIR)/$(TEST_DIR)/e2e/" -- \
		-e2e.artifacts-folder="$(ARTIFACTS)" \
		-e2e.config="$(E2E_CONF_FILE_ENVSUBST)" \
		-e2e.skip-resource-cleanup=$(SKIP_CLEANUP) \
		-e2e.keep-test-environment=$(KEEP_TEST_ENV) \
		-e2e.trigger-ephemeral-test=$(EPHEMERAL_TEST) \
		-e2e.use-existing-cluster=$(SKIP_CREATE_MGMT_CLUSTER)

	rm $(E2E_CONF_FILE_ENVSUBST)

## --------------------------------------
## Build
## --------------------------------------

.PHONY: build
build: binaries build-api build-e2e ## Builds all CAPM3 modules

.PHONY: binaries
binaries: manager ## Builds and installs all binaries

.PHONY: manager
manager: ## Build manager binary.
	go build -o $(BIN_DIR)/manager .

# Check that api package can be built
.PHONY: build-api
build-api:
	cd $(APIS_DIR) && go build ./...

# Check that e2e package can be built
.PHONY: build-e2e
build-e2e:
	cd $(TEST_DIR) && go build ./...

## --------------------------------------
## Tooling Binaries
## --------------------------------------

$(CLUSTERCTL): go.mod ## Build clusterctl binary.
	go build -o $(BIN_DIR)/clusterctl sigs.k8s.io/cluster-api/cmd/clusterctl

$(CONTROLLER_GEN): $(TOOLS_DIR)/go.mod # Build controller-gen from tools folder.
	cd $(TOOLS_DIR) && go build -tags=tools -o $(BIN_DIR)/controller-gen sigs.k8s.io/controller-tools/cmd/controller-gen

$(GOLANGCI_LINT): $(TOOLS_DIR)/go.mod # Build golangci-lint from tools folder.
	cd $(TOOLS_DIR) && go build -tags=tools -o $(BIN_DIR)/golangci-lint github.com/golangci/golangci-lint/cmd/golangci-lint

$(MOCKGEN): $(TOOLS_DIR)/go.mod # Build mockgen from tools folder.
	cd $(TOOLS_DIR) && go build -tags=tools -o $(BIN_DIR)/mockgen github.com/golang/mock/mockgen

$(CONVERSION_GEN): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR) && go build -tags=tools -o $(BIN_DIR)/conversion-gen k8s.io/code-generator/cmd/conversion-gen

$(KUBEBUILDER): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR) && ./install_kubebuilder.sh

$(SETUP_ENVTEST): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR) && \
	go build -tags=tools -o $(BIN_DIR)/setup-envtest sigs.k8s.io/controller-runtime/tools/setup-envtest

$(GINKGO): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR) && go get github.com/onsi/ginkgo/v2/ginkgo@$(GINGKO_VER)
	cd $(TOOLS_DIR) && go build -tags=tools -o $(BIN_DIR)/ginkgo github.com/onsi/ginkgo/v2/ginkgo

.PHONY: $(KUSTOMIZE)
$(KUSTOMIZE): # Download kustomize using hack script into tools folder.
	hack/ensure-kustomize.sh

$(ENVSUBST): ## Build envsubst from tools folder.
	rm -f $(TOOLS_BIN_DIR)/$(ENVSUBST_BIN)*
	cd $(TOOLS_DIR) && go build -tags=tools -o $(BIN_DIR)/envsubst-drone github.com/drone/envsubst/cmd/envsubst
	ln -sf envsubst-drone $(TOOLS_BIN_DIR)/$(ENVSUBST_BIN)

.PHONY: $(ENVSUBST_BIN)
$(ENVSUBST_BIN): $(ENVSUBST) ## Build envsubst from tools folder.

## --------------------------------------
## Linting
## --------------------------------------

.PHONY: lint
lint: $(GOLANGCI_LINT) ## Lint codebase
	$(GOLANGCI_LINT) run -v --timeout=10m
	cd $(APIS_DIR) && ../$(GOLANGCI_LINT) run -v --timeout=10m
	cd $(TEST_DIR) && ../$(GOLANGCI_LINT) run -v --timeout=10m

lint-full: $(GOLANGCI_LINT) ## Run slower linters to detect possible issues
	$(GOLANGCI_LINT) run -v --fast=false --timeout=30m
	cd $(APIS_DIR) && ../$(GOLANGCI_LINT) run -v --fast=false --timeout=30m
	cd $(TEST_DIR) && ../$(GOLANGCI_LINT) run -v --fast=false --timeout=30m

# Run go fmt against code
fmt:
	go fmt ./controllers/... ./baremetal/... .
	cd $(APIS_DIR) && go fmt  ./...
	cd $(TEST_DIR) && go fmt  ./...

# Run go vet against code
vet:
	go vet ./controllers/... ./baremetal/... .
	cd $(APIS_DIR) && go vet  ./...
	cd $(TEST_DIR) && go fmt  ./...

# Run manifest validation
.PHONY: manifest-lint
manifest-lint:
	./hack/manifestlint.sh

## --------------------------------------
## Generate
## --------------------------------------

.PHONY: modules
modules: ## Runs go mod to ensure proper vendoring.
	go mod tidy
	go mod verify
	cd $(TOOLS_DIR) && go mod tidy
	cd $(TOOLS_DIR) && go mod verify
	cd $(APIS_DIR) && go mod tidy
	cd $(APIS_DIR) && go mod verify
	cd $(TEST_DIR) && go mod tidy
	cd $(TEST_DIR) && go mod verify

.PHONY: generate
generate: ## Generate code
	$(MAKE) generate-go
	$(MAKE) generate-manifests

.PHONY: generate-go
generate-go: $(CONTROLLER_GEN) $(MOCKGEN) $(CONVERSION_GEN) $(KUBEBUILDER) $(KUSTOMIZE) ## Runs Go related generate targets
	go generate ./...
	cd $(APIS_DIR) && go generate ./...

	cd ./api && ../$(CONTROLLER_GEN) \
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

	$(CONVERSION_GEN) \
		--input-dirs=./api/v1alpha5 \
		--output-file-base=zz_generated.conversion  $(CONVERSION_GEN_OUTPUT_BASE) \
		--go-header-file=./hack/boilerplate/boilerplate.generatego.txt

.PHONY: generate-manifests
generate-manifests: $(CONTROLLER_GEN) ## Generate manifests e.g. CRD, RBAC etc.
	cd $(APIS_DIR) && ../$(CONTROLLER_GEN) \
		paths=./... \
		crd:crdVersions=v1 \
		output:crd:dir=../$(CRD_ROOT) \
		output:webhook:dir=../$(WEBHOOK_ROOT) \
		webhook
	$(CONTROLLER_GEN) \
		paths=./controllers/... \
		output:rbac:dir=$(RBAC_ROOT) \
		rbac:roleName=manager-role

.PHONY: generate-examples
generate-examples: $(KUSTOMIZE) clean-examples ## Generate examples configurations to run a cluster.
	./examples/generate.sh

## --------------------------------------
## Docker
## --------------------------------------

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

deploy-examples:
	kubectl apply -f ./examples/_out/metal3plane.yaml
	kubectl apply -f ./examples/_out/cluster.yaml
	kubectl apply -f ./examples/_out/machinedeployment.yaml
	kubectl apply -f ./examples/_out/controlplane.yaml

delete-examples:
	kubectl delete -f ./examples/_out/machinedeployment.yaml || true
	kubectl delete -f ./examples/_out/controlplane.yaml || true
	kubectl delete -f ./examples/_out/cluster.yaml || true
	kubectl delete -f ./examples/_out/metal3plane.yaml || true


## --------------------------------------
## Release
## --------------------------------------

## latest git tag for the commit, e.g., v1.4.0
RELEASE_TAG ?= $(shell git describe --abbrev=0 2>/dev/null)
ifneq (,$(findstring -,$(RELEASE_TAG)))
    PRE_RELEASE=true
endif
# the previous release tag, e.g., v1.4.0, excluding pre-release tags
PREVIOUS_TAG ?= $(shell git tag -l | grep -E "^v[0-9]+\.[0-9]+\.[0-9]+$$" | sort -V | grep -B1 $(RELEASE_TAG) | head -n 1 2>/dev/null)
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
	if [ -n "${PRE_RELEASE}" ]; then \
	echo ":rotating_light: This is a RELEASE CANDIDATE. Use it only for testing purposes. If you find any bugs, file an [issue](https://github.com/metal3-io/cluster-api-provider-metal3/issues/new/)." > $(RELEASE_NOTES_DIR)/$(RELEASE_TAG).md; \
	else \
	go run ./hack/tools/release/notes.go --from=$(PREVIOUS_TAG) > $(RELEASE_NOTES_DIR)/$(RELEASE_TAG).md; \
	fi

.PHONY: release
release:
	@if [ -z "${RELEASE_TAG}" ]; then echo "RELEASE_TAG is not set"; exit 1; fi
	@if ! [ -z "$$(git status --porcelain)" ]; then echo "You have uncommitted changes"; exit 1; fi
	git checkout "${RELEASE_TAG}"
	MANIFEST_IMG=$(CONTROLLER_IMG) MANIFEST_TAG=$(RELEASE_TAG) $(MAKE) set-manifest-image
	$(MAKE) release-manifests
	$(MAKE) release-notes

## --------------------------------------
## Development
## --------------------------------------

.PHONY: create-cluster
create-cluster: $(CLUSTERCTL) ## Create a development Kubernetes cluster using examples
	$(CLUSTERCTL) \
	create cluster -v 4 \
	--bootstrap-flags="name=capm3" \
	--bootstrap-type kind \
	-m ./examples/_out/controlplane.yaml \
	-c ./examples/_out/cluster.yaml \
	-p ./examples/_out/provider-components.yaml \
	-a ./examples/addons.yaml


.PHONY: create-cluster-management
create-cluster-management: $(CLUSTERCTL) ## Create a development Kubernetes cluster in a KIND management cluster.
	kind create cluster --name=capm3
	# Apply provider-components.
	kubectl \
		--kubeconfig=$$(kind get kubeconfig-path --name="capm3") \
		create -f examples/_out/provider-components.yaml
	# Create Cluster.
	kubectl \
		--kubeconfig=$$(kind get kubeconfig-path --name="capm3") \
		create -f examples/_out/cluster.yaml
	# Create control plane machine.
	kubectl \
		--kubeconfig=$$(kind get kubeconfig-path --name="capm3") \
		create -f examples/_out/controlplane.yaml
	# Get KubeConfig using clusterctl.
	$(CLUSTERCTL) \
		alpha phases get-kubeconfig -v=3 \
		--kubeconfig=$$(kind get kubeconfig-path --name="capm3") \
		--namespace=default \
		--cluster-name=test1
	# Apply addons on the target cluster, waiting for the control-plane to become available.
	$(CLUSTERCTL) \
		alpha phases apply-addons -v=3 \
		--kubeconfig=./kubeconfig \
		-a examples/addons.yaml
	# Create a worker node with MachineDeployment.
	kubectl \
		--kubeconfig=$$(kind get kubeconfig-path --name="capm3") \
		create -f examples/_out/machinedeployment.yaml

.PHONY: delete-cluster
delete-workload-cluster: $(CLUSTERCTL) ## Deletes the development Kubernetes Cluster "test1"
	$(CLUSTERCTL) \
	delete cluster -v 4 \
	--bootstrap-type kind \
	--bootstrap-flags="name=clusterapi" \
	--cluster test1 \
	--kubeconfig ./kubeconfig \
	-p ./examples/_out/provider-components.yaml \

## --------------------------------------
## Tilt / Kind
## --------------------------------------

.PHONY: kind-create
kind-create: ## create capm3 kind cluster if needed
	./hack/kind_with_registry.sh

.PHONY: tilt-settings
tilt-settings:
	./hack/gen_tilt_settings.sh

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
