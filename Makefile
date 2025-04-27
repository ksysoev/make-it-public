test:
	go test --race ./...

lint:
	golangci-lint run

mocks:
	mockery

mod-tidy:
	go mod tidy

fmt:
	go fmt ./...
