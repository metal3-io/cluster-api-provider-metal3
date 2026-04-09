# Build the extension binary
ARG BUILD_IMAGE=docker.io/golang:1.25.8@sha256:779b230b2508037a8095c9e2d223a6405f8426e12233b694dbae50197b9f6d04
ARG BASE_IMAGE=gcr.io/distroless/static:nonroot@sha256:9ecc53c269509f63c69a266168e4a687c7eb8c0cfd753bd8bfcaa4f58a90876f

FROM $BUILD_IMAGE AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
COPY api/ api/
COPY test/go.mod test/go.mod
COPY test/go.sum test/go.sum

# Cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download
RUN cd test && go mod download

# Copy the go source
COPY test/extension/ test/extension/

# Build
RUN cd test/extension && \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -a -o /workspace/extension \
    -ldflags '-extldflags "-static"' \
    .


FROM $BASE_IMAGE
WORKDIR /
COPY --from=builder /workspace/extension .
USER 65532:65532

ENTRYPOINT ["/extension"]
