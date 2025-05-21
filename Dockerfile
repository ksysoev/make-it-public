FROM golang:1.24-alpine AS builder

ARG MIT_SERVER=${MIT_SERVER}
ARG VERSION=${VERSION}
ARG GOOS=${PLATFORM_OS}
ARG GOARCH=${PLATFORM_ARCH}

WORKDIR /app

COPY . .
RUN go mod download

RUN \
    if [ -z "$GOOS" || -z "$GOARCH" ]; then \
        echo "GOOS and GOARCH are not set, building cross platform"; \
        CGO_ENABLED=0 go build -o mit -ldflags "-X main.defaultServer=$MIT_SERVER -X main.version=$VERSION" ./cmd/mit/main.go \
    else \
        echo "Building for $GOOS/$GOARCH"; \
        CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build -o mit -ldflags "-X main.defaultServer=$MIT_SERVER -X main.version=$VERSION" ./cmd/mit/main.go \
    fi

FROM scratch

COPY --from=builder /app/mit .
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 8080 8081 8082

ENTRYPOINT ["/mit"]
