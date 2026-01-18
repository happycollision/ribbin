.PHONY: build install test test-coverage test-integration clean

BINARY_NAME=ribbin
BUILD_DIR=bin
TEST_IMAGE=ribbin-test

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/ribbin

install: build
	go install ./cmd/ribbin

# Run all tests in Docker container (safe - doesn't modify host system)
test:
	docker build -f Dockerfile.test -t $(TEST_IMAGE) .
	docker run --rm $(TEST_IMAGE)

# Run tests with coverage report
test-coverage:
	docker build -f Dockerfile.test -t $(TEST_IMAGE) .
	docker run --rm $(TEST_IMAGE) go test -cover ./...

# Run integration tests
test-integration:
	docker build -f Dockerfile.test -t $(TEST_IMAGE) .
	docker run --rm $(TEST_IMAGE) go test -tags=integration -v ./...

# Run tests interactively (for debugging)
test-shell:
	docker build -f Dockerfile.test -t $(TEST_IMAGE) .
	docker run --rm -it $(TEST_IMAGE) sh

clean:
	rm -rf $(BUILD_DIR)
	go clean
