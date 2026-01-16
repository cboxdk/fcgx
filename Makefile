.PHONY: test test-unit test-integration lint clean up down

# Default PHP version for local testing
PHP_VERSION ?= 8.3

# Run all tests
test: test-unit test-integration

# Run unit tests only (no PHP-FPM required)
test-unit:
	go test -short -race -v ./...

# Run integration tests (requires PHP-FPM)
test-integration: up
	@echo "Waiting for PHP-FPM..."
	@for i in $$(seq 1 30); do nc -z localhost 9000 2>/dev/null && break || sleep 1; done
	go test -race -v ./...

# Run tests against all PHP versions
test-all-php:
	@for v in 8.0 8.1 8.2 8.3 8.4 8.5; do \
		echo "Testing with PHP $$v..."; \
		PHP_VERSION=$$v $(MAKE) down up test-integration down || exit 1; \
	done

# Start PHP-FPM container
up:
	PHP_VERSION=$(PHP_VERSION) docker compose up -d

# Stop PHP-FPM container
down:
	docker compose down

# Run linter
lint:
	golangci-lint run ./...

# Clean up
clean: down
	rm -f coverage.txt

# Show coverage report
coverage:
	go test -short -coverprofile=coverage.txt -covermode=atomic ./...
	go tool cover -html=coverage.txt
