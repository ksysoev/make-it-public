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
	rm -rf ./pkg/api/docs && swag init -g ./pkg/api/api.go -o ./pkg/api/docs -ot go

docker-up:
	docker-compose up --build
