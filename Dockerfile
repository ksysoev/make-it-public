FROM golang:1.24.2 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o mitserver ./cmd/mitserver

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/mitserver .
COPY ./runtime/config.yaml ./runtime/config.yaml
RUN chmod +x ./mitserver

EXPOSE 8080 8081 8082

CMD ["./mitserver", "serve", "all", "config", "./runtime/config.yaml"]
