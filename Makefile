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
TOOLS_BIN_DIR := $(TOOLS_DIR)/bin
BIN_DIR := bin

# Set --output-base for conversion-gen if we are not within GOPATH
ifneq ($(abspath $(ROOT_DIR)),$(shell go env GOPATH)/src/sigs.k8s.io/cluster-api-provider-metal3)
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
RELEASE_NOTES_BIN := bin/release-notes
RELEASE_NOTES := $(TOOLS_DIR)/$(RELEASE_NOTES_BIN)
ENVSUBST_BIN := envsubst
ENVSUBST := $(TOOLS_BIN_DIR)/$(ENVSUBST_BIN)-drone

# Define Docker related variables. Releases should modify and double check these vars.
# REGISTRY ?= gcr.io/$(shell gcloud config get-value project)
REGISTRY ?= quay.io/metal3-io
STAGING_REGISTRY := quay.io/metal3-io
PROD_REGISTRY := quay.io/metal3-io
IMAGE_NAME ?= cluster-api-provider-metal3
CONTROLLER_IMG ?= $(REGISTRY)/$(IMAGE_NAME)
BMO_IMAGE_NAME ?= baremetal-operator
BMO_CONTROLLER_IMG ?= $(REGISTRY)/$(BMO_IMAGE_NAME)
TAG ?= v1alpha4
BMO_TAG ?= capm3-$(TAG)
ARCH ?= amd64
ALL_ARCH = amd64 arm arm64 ppc64le s390x

# Allow overriding manifest generation destination directory
MANIFEST_ROOT ?= config
CRD_ROOT ?= $(MANIFEST_ROOT)/crd/bases
METAL3_CRD_ROOT ?= $(MANIFEST_ROOT)/crd/metal3
WEBHOOK_ROOT ?= $(MANIFEST_ROOT)/webhook
RBAC_ROOT ?= $(MANIFEST_ROOT)/rbac

# Allow overriding the imagePullPolicy
PULL_POLICY ?= IfNotPresent

## --------------------------------------
## Help
## --------------------------------------

help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

## --------------------------------------
## Testing
## --------------------------------------

.PHONY: testprereqs
testprereqs: $(KUBEBUILDER) $(KUSTOMIZE)

.PHONY: test
test: testprereqs fmt lint ## Run tests
	source ./hack/fetch_ext_bins.sh; fetch_tools; setup_envs; go test -v ./api/... ./controllers/... ./baremetal/... -coverprofile ./cover.out

.PHONY: test-integration
test-integration: ## Run integration tests
	source ./hack/fetch_ext_bins.sh; fetch_tools; setup_envs; go test -v -tags=integration ./test/integration/...

.PHONY: test-e2e
test-e2e: ## Run e2e tests
	PULL_POLICY=IfNotPresent $(MAKE) docker-build
	go test -v -tags=e2e -timeout=1h ./test/e2e/... -args --managerImage $(CONTROLLER_IMG)-$(ARCH):$(TAG)

GINKGO_NOCOLOR ?= false
ARTIFACTS ?= $(ROOT_DIR)/_artifacts
E2E_CONF_FILE ?= $(ROOT_DIR)/test/e2e/config/e2e_conf.yaml
E2E_CONF_FILE_ENVSUBST ?= $(ROOT_DIR)/test/e2e/config/e2e_conf_envsubst.yaml
SKIP_CLEANUP ?= false
SKIP_CREATE_MGMT_CLUSTER ?= true

.PHONY: e2e-tests
e2e-tests: ## Run e2e tests with capi e2e testing framework
	docker pull quay.io/metal3-io/cluster-api-provider-metal3
	docker pull quay.io/metal3-io/baremetal-operator
	docker pull quay.io/metal3-io/ip-address-manager
	make envsubst
	$(ENVSUBST) < $(E2E_CONF_FILE) > $(E2E_CONF_FILE_ENVSUBST)
	time go test -v -timeout 24h -tags=e2e ./test/e2e/... -args \
		-ginkgo.v -ginkgo.trace -ginkgo.progress -ginkgo.noColor=$(GINKGO_NOCOLOR) \
		-e2e.artifacts-folder="$(ARTIFACTS)" \
		-e2e.config="$(E2E_CONF_FILE_ENVSUBST)" \
		-e2e.skip-resource-cleanup=$(SKIP_CLEANUP) \
		-e2e.use-existing-cluster=$(SKIP_CREATE_MGMT_CLUSTER)
	rm $(E2E_CONF_FILE_ENVSUBST)

## --------------------------------------
## Binaries
## --------------------------------------

.PHONY: binaries
binaries: manager ## Builds and installs all binaries

.PHONY: manager
manager: ## Build manager binary.
	go build -o $(BIN_DIR)/manager .

## --------------------------------------
## Tooling Binaries
## --------------------------------------

$(CLUSTERCTL): go.mod ## Build clusterctl binary.
	go build -o $(BIN_DIR)/clusterctl sigs.k8s.io/cluster-api/cmd/clusterctl

$(CONTROLLER_GEN): $(TOOLS_DIR)/go.mod # Build controller-gen from tools folder.
	cd $(TOOLS_DIR); go build -tags=tools -o $(BIN_DIR)/controller-gen sigs.k8s.io/controller-tools/cmd/controller-gen

$(GOLANGCI_LINT): $(TOOLS_DIR)/go.mod # Build golangci-lint from tools folder.
	cd $(TOOLS_DIR); go build -tags=tools -o $(BIN_DIR)/golangci-lint github.com/golangci/golangci-lint/cmd/golangci-lint

$(MOCKGEN): $(TOOLS_DIR)/go.mod # Build mockgen from tools folder.
	cd $(TOOLS_DIR); go build -tags=tools -o $(BIN_DIR)/mockgen github.com/golang/mock/mockgen

$(CONVERSION_GEN): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go build -tags=tools -o $(BIN_DIR)/conversion-gen k8s.io/code-generator/cmd/conversion-gen

$(KUBEBUILDER): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); ./install_kubebuilder.sh

.PHONY: $(KUSTOMIZE)
$(KUSTOMIZE): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); ./install_kustomize.sh

$(ENVSUBST): ## Build envsubst from tools folder.
	rm -f $(TOOLS_BIN_DIR)/$(ENVSUBST_BIN)*
	cd $(TOOLS_DIR) && go build -tags=tools -o $(BIN_DIR)/envsubst-drone github.com/drone/envsubst/cmd/envsubst
	ln -sf envsubst-drone $(TOOLS_BIN_DIR)/$(ENVSUBST_BIN)

.PHONY: $(ENVSUBST_BIN)
$(ENVSUBST_BIN): $(ENVSUBST) ## Build envsubst from tools folder.


$(RELEASE_NOTES) : $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR) && go build -tags=tools -o $(RELEASE_NOTES_BIN) ./release

## --------------------------------------
## Linting
## --------------------------------------

.PHONY: lint
lint: $(GOLANGCI_LINT) ## Lint codebase
	$(GOLANGCI_LINT) run -v --timeout=10m

lint-full: $(GOLANGCI_LINT) ## Run slower linters to detect possible issues
	$(GOLANGCI_LINT) run -v --fast=false --timeout=30m

# Run go fmt against code
fmt:
	go fmt ./api/... ./controllers/... ./baremetal/... .

# Run go vet against code
vet:
	go vet ./api/... ./controllers/... ./baremetal/... .

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
	cd $(TOOLS_DIR); go mod tidy

.PHONY: generate
generate: ## Generate code
	$(MAKE) generate-go
	$(MAKE) generate-manifests

.PHONY: generate-go
generate-go: $(CONTROLLER_GEN) $(MOCKGEN) $(CONVERSION_GEN) $(KUBEBUILDER) $(KUSTOMIZE) ## Runs Go related generate targets
	go generate ./...
	$(CONTROLLER_GEN) \
		paths=./api/... \
		object:headerFile=./hack/boilerplate/boilerplate.generatego.txt

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
	  -destination=./baremetal/mocks/zz_generated.metal3data_manager.go \
	  -source=./baremetal/metal3data_manager.go \
		-package=baremetal_mocks \
		-copyright_file=./hack/boilerplate/boilerplate.generatego.txt \
		DataManagerInterface

	$(MOCKGEN) \
	  -destination=./baremetal/mocks/zz_generated.manager_factory.go \
	  -source=./baremetal/manager_factory.go \
		-package=baremetal_mocks \
		-copyright_file=./hack/boilerplate/boilerplate.generatego.txt \
		ManagerFactoryInterface

	$(CONVERSION_GEN) \
		--input-dirs=./api/v1alpha3 \
		--output-file-base=zz_generated.conversion  $(CONVERSION_GEN_OUTPUT_BASE) \
		--go-header-file=./hack/boilerplate/boilerplate.generatego.txt

.PHONY: generate-manifests
generate-manifests: $(CONTROLLER_GEN) ## Generate manifests e.g. CRD, RBAC etc.
	$(CONTROLLER_GEN) \
		paths=./api/... \
		crd:crdVersions=v1 \
		output:crd:dir=$(CRD_ROOT) \
		output:webhook:dir=$(WEBHOOK_ROOT) \
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
	sed -i'' -e 's@image: .*@image: '"${MANIFEST_IMG}:$(MANIFEST_TAG)"'@' ./config/manager/manager_image_patch.yaml

.PHONY: set-manifest-image-bmo
set-manifest-image-bmo:
	$(info Updating kustomize image patch file for baremetal-operator)
	sed -i'' -e 's@image: .*@image: '"${MANIFEST_IMG_BMO}:$(MANIFEST_TAG_BMO)"'@' ./config/bmo/bmo_image_patch.yaml

.PHONY: set-manifest-image-ipam
set-manifest-image-ipam:
	$(info Updating kustomize image patch file for IPAM controller)
	sed -i'' -e 's@image: .*@image: '"${MANIFEST_IMG_IPAM}:$(MANIFEST_TAG_IPAM)"'@' ./config/ipam/image_patch.yaml

.PHONY: set-manifest-pull-policy
set-manifest-pull-policy:
	$(info Updating kustomize pull policy file for manager resource)
	sed -i'' -e 's@imagePullPolicy: .*@imagePullPolicy: '"$(PULL_POLICY)"'@' ./config/manager/manager_pull_policy_patch.yaml
	sed -i'' -e 's@imagePullPolicy: .*@imagePullPolicy: '"$(PULL_POLICY)"'@' ./config/bmo/bmo_pull_policy_patch.yaml
	sed -i'' -e 's@imagePullPolicy: .*@imagePullPolicy: '"$(PULL_POLICY)"'@' ./config/ipam/pull_policy_patch.yaml
## --------------------------------------
## Deploying
## --------------------------------------

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./main.go

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: generate-examples
	kubectl apply -f examples/_out/cert-manager.yaml
	kubectl wait --for=condition=Available --timeout=300s -n cert-manager deployment cert-manager
	kubectl wait --for=condition=Available --timeout=300s -n cert-manager deployment cert-manager-cainjector
	kubectl wait --for=condition=Available --timeout=300s -n cert-manager deployment cert-manager-webhook
	kubectl apply -f examples/_out/provider-components.yaml

deploy-examples: generate-examples
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

RELEASE_TAG := $(shell git describe --abbrev=0 2>/dev/null)
RELEASE_DIR := out

$(RELEASE_DIR):
	mkdir -p $(RELEASE_DIR)/

.PHONY: release
release: clean-release  ## Builds and push container images using the latest git tag for the commit.
	@if [ -z "${RELEASE_TAG}" ]; then echo "RELEASE_TAG is not set"; exit 1; fi
	# Push the release image to the staging bucket first.
	#REGISTRY=$(STAGING_REGISTRY) TAG=$(RELEASE_TAG) \
	#	$(MAKE) docker-build-all docker-push-all
	# Set the manifest image to the production bucket.
	MANIFEST_IMG=$(PROD_REGISTRY)/$(IMAGE_NAME) MANIFEST_TAG=$(RELEASE_TAG) \
		$(MAKE) set-manifest-image
	# TODO : this is temporarily, as long as we don't have BMO releases, we use compatibility
	# tags. The "capm3-<release-tag>" tag must be created on current BMO master branch
	MANIFEST_IMG_BMO=$(PROD_REGISTRY)/$(BMO_IMAGE_NAME) MANIFEST_TAG_BMO=capm3-$(RELEASE_TAG) \
		$(MAKE) set-manifest-image-bmo
	PULL_POLICY=IfNotPresent $(MAKE) set-manifest-pull-policy

	$(MAKE) release-manifests
	$(MAKE) release-binaries

.PHONY: release-manifests
release-manifests: $(KUSTOMIZE) $(RELEASE_DIR) ## Builds the manifests to publish with a release
	$(KUSTOMIZE) build config > $(RELEASE_DIR)/infrastructure-components.yaml
	cp metadata.yaml $(RELEASE_DIR)/metadata.yaml
	cp examples/clusterctl-templates/clusterctl-cluster.yaml $(RELEASE_DIR)/cluster-template.yaml
	cp examples/clusterctl-templates/example_variables.rc $(RELEASE_DIR)/example_variables.rc

.PHONY: release-binaries
release-binaries: ## Builds the binaries to publish with a release

.PHONY: release-binary
release-binary: $(RELEASE_DIR)
	docker run \
		--rm \
		-e CGO_ENABLED=0 \
		-e GOOS=$(GOOS) \
		-e GOARCH=$(GOARCH) \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		golang:1.15.3 \
		go build -a -ldflags '-extldflags "-static"' \
		-o $(RELEASE_DIR)/$(notdir $(RELEASE_BINARY))-$(GOOS)-$(GOARCH) $(RELEASE_BINARY)

.PHONY: release-staging
release-staging: ## Builds and push container images to the staging bucket.
	REGISTRY=$(STAGING_REGISTRY) $(MAKE) docker-build-all docker-push-all release-tag-latest


.PHONY: release-tag-latest
release-tag-latest: ## Adds the latest tag to the last build tag.
	## TODO(vincepri): Only do this when we're on master.
	gcloud container images add-tag $(CONTROLLER_IMG):$(TAG) $(CONTROLLER_IMG):latest

.PHONY: release-notes
release-notes: $(RELEASE_NOTES)  ## Generates a release notes template to be used with a release.
	$(RELEASE_NOTES)

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
