test:
	go test --race ./...

test-e2e:
	go test -v -run TestServerE2E ./pkg/cmd/

test-e2e-with-redis:
	@echo "Cleaning up any existing Redis container..."
	@docker rm -f mit-test-redis > /dev/null 2>&1 || true
	@echo "Starting Redis container..."
	@docker run -d --rm --name mit-test-redis -p 6379:6379 redis:alpine > /dev/null
	@echo "Waiting for Redis to be ready..."
	@timeout 30 sh -c 'until docker exec mit-test-redis redis-cli ping 2>/dev/null | grep -q PONG; do sleep 0.5; done' || (echo "Redis failed to start"; docker rm -f mit-test-redis > /dev/null 2>&1; exit 1)
	@echo "Running E2E tests..."
	@go test -v -run TestServerE2E ./pkg/cmd/ || (docker rm -f mit-test-redis > /dev/null 2>&1; exit 1)
	@echo "Stopping Redis container..."
	@docker rm -f mit-test-redis > /dev/null 2>&1

test-coverage:
	go test -coverprofile=coverage.out -covermode=atomic -v -race ./...

build:
	go build -v ./...

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
