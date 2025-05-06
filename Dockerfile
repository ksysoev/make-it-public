FROM golang:1.24.2 AS builder

WORKDIR /app

COPY . .
RUN go mod download

RUN CGO_ENABLED=0 go build -o mit ./cmd/mit/main.go

FROM scratch

COPY --from=builder /app/mit .
COPY ./runtime/config.yaml /runtime/config.yaml

EXPOSE 8080 8081 8082

ENTRYPOINT ["/mit"]
CMD ["server", "run", "all", "--config", "runtime/config.yaml"]
