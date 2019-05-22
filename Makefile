
# Image URL to use all building/pushing image targets
IMG ?= controller:latest

.PHONY: build
build:
	@mkdir -p bin
	go build -o bin/machine-controller-manager ./cmd/manager
	go build -o bin/manager ./vendor/sigs.k8s.io/cluster-api/cmd/manager

all: test manager

# Run tests
test: testprereqs generate fmt vet unit

.PHONY: testprereqs
testprereqs:
	@if [ ! -d /usr/local/kubebuilder ] ; then echo "kubebuilder not found.  See docs/dev/setup.md" && exit 1 ; fi
	@if ! which kustomize >/dev/null 2>&1 ; then echo "kustomize not found.  See docs/dev/setup.md" && exit 1 ; fi

unit: manifests
	go test ./pkg/... ./cmd/... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager github.com/metal3-io/cluster-api-provider-baremetal/cmd/manager

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./cmd/manager/main.go

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	cat provider-components.yaml | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go all
	kustomize build config/ > provider-components.yaml
	echo "---" >> provider-components.yaml
	cd vendor && kustomize build sigs.k8s.io/cluster-api/config/default/ | \
		sed -e 's/namespace: cluster-api-system/namespace: metal3/' >> ../provider-components.yaml

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

# Generate code
generate:
ifndef GOPATH
	$(error GOPATH not defined, please define GOPATH. Run "go help gopath" to learn more about GOPATH)
endif
	go generate ./pkg/... ./cmd/...

# Build the docker image
docker-build: test
	docker build . -t ${IMG}
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"${IMG}"'@' ./config/default/manager_image_patch.yaml

# Push the docker image
docker-push:
	docker push ${IMG}
