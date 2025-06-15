FROM golang:1.24.4-alpine AS builder

ARG MIT_SERVER=${MIT_SERVER}
ARG VERSION=${VERSION}

WORKDIR /app

COPY . .
RUN go mod download

RUN CGO_ENABLED=0 go build -o mit -ldflags "-X main.defaultServer=$MIT_SERVER -X main.version=$VERSION" ./cmd/mit/main.go

FROM scratch

COPY --from=builder /app/mit .
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 8080 8081 8082

ENTRYPOINT ["/mit"]
