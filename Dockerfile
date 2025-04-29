FROM golang:1.24.2 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 go build -o mitserver ./cmd/mitserver

FROM scratch

COPY --from=builder /app/mitserver .
COPY ./runtime/config.yaml /runtime/config.yaml

EXPOSE 8080 8081 8082

ENTRYPOINT ["/mitserver"]
CMD ["serve", "all", "--config", "runtime/config.yaml"]
