# Copyright 2021 The Kubernetes Authors.
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

# Support FROM override
ARG BUILD_IMAGE=docker.io/golang:1.24.9@sha256:5034fa44b36163a4109b71ed75c67dbdcb52c3cd9750953befe00054315d9fd2
ARG BASE_IMAGE=gcr.io/distroless/static:nonroot@sha256:9ecc53c269509f63c69a266168e4a687c7eb8c0cfd753bd8bfcaa4f58a90876f

# Build the manager binary on golang image
FROM $BUILD_IMAGE AS builder
WORKDIR /workspace

# Run this with docker build --build_arg $(go env GOPROXY) to override the goproxy
ARG goproxy=https://proxy.golang.org
ENV GOPROXY=$goproxy

# Copy the Go Modules manifests
COPY go.mod go.sum ./
COPY api/go.mod api/go.sum api/
COPY test/go.mod test/go.sum test/
# Cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the sources
COPY main.go main.go
COPY api/ api/
COPY baremetal/ baremetal/
COPY controllers/ controllers/
COPY internal/ internal/

# Build
ARG ARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} \
    go build -a -ldflags '-extldflags "-static"' \
    -o manager .

# Copy the controller-manager into a thin image
FROM $BASE_IMAGE

# image.version is set during image build by automation
LABEL org.opencontainers.image.authors="metal3-dev@googlegroups.com"
LABEL org.opencontainers.image.description="This is the image for the Cluster API Provider Metal3"
LABEL org.opencontainers.image.documentation="https://book.metal3.io/capm3/introduction"
LABEL org.opencontainers.image.licenses="Apache License 2.0"
LABEL org.opencontainers.image.title="Cluster API Provider Metal3"
LABEL org.opencontainers.image.url="https://github.com/metal3-io/cluster-api-provider-metal3"
LABEL org.opencontainers.image.vendor="Metal3-io"

WORKDIR /
COPY --from=builder /workspace/manager .
# Use uid of nonroot user (65532) because kubernetes expects numeric user when applying pod security policies
USER 65532
ENTRYPOINT ["/manager"]
