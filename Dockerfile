FROM golang:1.24.2 AS builder

WORKDIR /app

COPY . .
RUN go mod download

RUN CGO_ENABLED=0 go build -o mit ./cmd/mit/main.go

FROM scratch

COPY --from=builder /app/mit .
COPY ./docs/swagger.json /docs/swagger.json

EXPOSE 8080 8081 8082

ENTRYPOINT ["/mit"]
