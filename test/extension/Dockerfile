# Build the extension binary
FROM golang:1.24.0 AS builder

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


FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/extension .
USER 65532:65532

ENTRYPOINT ["/extension"]
