# syntax=docker/dockerfile:1
FROM golang:1.19-alpine AS builder

LABEL stage=gobuilder

ARG VERSION=v0.0.1
ENV VERSION=$VERSION
ENV CGO_ENABLED 0
ENV GOOS linux

RUN apk update --no-cache && apk add --no-cache tzdata

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download -x
RUN go install github.com/go-delve/delve/cmd/dlv@latest

COPY . .

RUN go build -gcflags="all=-N -l" -ldflags="-X hetzner-k3s/cmd/commands.Version=${VERSION}" -o hetzner-k3s cmd/hetzner-k3s/main.go

FROM alpine

COPY entrypoint.sh /entrypoint.sh

RUN apk add --no-cache --update bash \
    && BUILD_ARCH="$(apk --print-arch)" \
    && if [ "${BUILD_ARCH}" = "aarch64" ]; \
        then BUILD_ARCH="arm64"; \
        else BUILD_ARCH="amd64"; \
       fi \
    && wget -q -O /usr/bin/kubectl "https://dl.k8s.io/release/$(wget -q -O- https://dl.k8s.io/release/stable.txt)/bin/linux/${BUILD_ARCH}/kubectl" \
    && chmod +x /usr/bin/kubectl \
    && chmod +x /entrypoint.sh

COPY --from=builder /app/hetzner-k3s /usr/bin/hetzner-k3s
COPY --from=builder /go/bin/dlv /

EXPOSE 40000

ENTRYPOINT ["/dlv", "--listen=:40000", "--headless=true", "--api-version=2", "--accept-multiclient", "exec", "/usr/bin/hetzner-k3s", "--"]
CMD ["help"]
