.PHONY: build install test test-coverage test-integration test-host benchmark benchmark-grep benchmark-all benchmark-full scenario clean

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

# Run tests on host (DANGEROUS - only for debugging, may modify system files)
# Requires explicit opt-in via environment variable
test-host:
	RIBBIN_DANGEROUSLY_ALLOW_HOST_TESTS=1 go test ./...

# Run benchmark to measure shim overhead on cat (10k iterations, fast command)
benchmark:
	docker build -f Dockerfile.test -t $(TEST_IMAGE) .
	docker run --rm $(TEST_IMAGE) go test -tags=integration -bench=BenchmarkShimOverhead -benchtime=10000x -run=^$$ ./internal

# Run benchmark to measure shim overhead on grep (1k iterations, slower command)
benchmark-grep:
	docker build -f Dockerfile.test -t $(TEST_IMAGE) .
	docker run --rm $(TEST_IMAGE) go test -tags=integration -bench=BenchmarkShimOverheadGrep -benchtime=1000x -run=^$$ ./internal

# Run all benchmarks
benchmark-all:
	docker build -f Dockerfile.test -t $(TEST_IMAGE) .
	docker run --rm $(TEST_IMAGE) go test -tags=integration -bench=Benchmark -benchtime=1000x -run=^$$ ./internal

# Run full benchmark with cat (1 million iterations)
benchmark-full:
	docker build -f Dockerfile.test -t $(TEST_IMAGE) .
	docker run --rm $(TEST_IMAGE) go test -tags=integration -bench=BenchmarkShimOverhead -benchtime=1000000x -run=^$$ ./internal

# Interactive scenario for manual testing (runs in Docker)
# Builds ribbin, sets up a test project with shims, drops you into a shell
# Type 'exit' to leave - all artifacts are cleaned up automatically
#
# Usage:
#   make scenario              # Show scenario menu
#   make scenario SCENARIO=basic        # Run basic scenario directly
#   make scenario SCENARIO=local-dev-mode  # Run local dev mode scenario
scenario:
	docker build -f Dockerfile.scenario -t ribbin-scenario .
	-docker run --rm -it -e SCENARIO=$(SCENARIO) ribbin-scenario

clean:
	rm -rf $(BUILD_DIR)
	go clean
