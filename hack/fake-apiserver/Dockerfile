# Copyright 2024 The Kubernetes Authors.
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
ARG BUILD_IMAGE=docker.io/golang:1.24.11@sha256:cf1272dbf972a94f39a81dcb9dc243a8d2f981e5dd3b5a5c965f6d9ab9268b26
ARG BASE_IMAGE=gcr.io/distroless/static:nonroot@sha256:9ecc53c269509f63c69a266168e4a687c7eb8c0cfd753bd8bfcaa4f58a90876f

# Build the fkas binary on golang image
FROM $BUILD_IMAGE AS base
WORKDIR /workspace

# Run this with docker build --build_arg $(go env GOPROXY) to override the goproxy
ARG goproxy=https://proxy.golang.org
ENV GOPROXY=$goproxy

# Copy the Go Modules manifests
COPY go.mod go.sum ./

# Cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Build Fkas
FROM base AS build-fkas

# Copy the sources
COPY cmd/metal3-fkas/*.go ./

# Build
ARG ARCH=amd64
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} \
    go build -a -ldflags '-extldflags "-static"' \
    -o fkas .

# Build fkas-reconciler
FROM base AS build-fkas-reconciler

# Copy the sources
COPY cmd/metal3-fkas-reconciler/*.go ./

# Build
ARG ARCH=amd64
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} \
    go build -a -ldflags '-extldflags "-static"' \
    -o reconciler .

# Copy the controller-manager into a thin image
FROM $BASE_IMAGE
WORKDIR /
# Use uid of nonroot user (65532) because kubernetes expects numeric user when applying pod security policies
COPY --from=build-fkas /workspace/fkas .
COPY --from=build-fkas-reconciler /workspace/reconciler .
USER 65532
ENTRYPOINT ["/fkas"]
