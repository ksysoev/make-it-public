test:
	go test --race ./...

lint:
	golangci-lint run

mocks:
	mockery

mod-tidy:
	go mod tidy

fmt:
	gofmt -w .

api-doc:
	rm -rf ./docs && swag init -g ./pkg/api/api.go

docker-up:
	docker-compose up --build
