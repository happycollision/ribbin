.PHONY: build install install-next test test-unit test-coverage test-host benchmark benchmark-grep benchmark-all benchmark-full scenario release clean copy-schemas

BINARY_NAME=ribbin
BUILD_DIR=bin
TEST_IMAGE=ribbin-test
INSTALL_DIR=$(HOME)/.local/bin
SCHEMA_SRC=schemas/v1
SCHEMA_DEST=internal/config/schemas/v1

# Copy schemas before build (required for go:embed)
copy-schemas:
	@mkdir -p $(SCHEMA_DEST)
	@cp $(SCHEMA_SRC)/*.json $(SCHEMA_DEST)/

build: copy-schemas
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/ribbin

install: build
	go install ./cmd/ribbin

# Build and install as ribbin-next to ~/.local/bin (bypasses local dev mode)
install-next: build
	mkdir -p $(INSTALL_DIR)
	cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/ribbin-next
	chmod +x $(INSTALL_DIR)/ribbin-next
	@echo "Installed ribbin-next to $(INSTALL_DIR)/ribbin-next"

# Run all tests in Docker container (safe - doesn't modify host system)
# Usage:
#   make test                    # Run all tests
#   make test RUN=TestNodeModules  # Run tests matching pattern
test:
	docker build -f Dockerfile.test -t $(TEST_IMAGE) .
ifdef RUN
	docker run --rm $(TEST_IMAGE) gotestsum --format testdox -- ./... -run "$(RUN)"
else
	docker run --rm $(TEST_IMAGE) gotestsum --format testdox -- ./...
endif

# Run unit tests only (faster, excludes internal/ integration tests)
test-unit:
	docker build -f Dockerfile.test -t $(TEST_IMAGE) .
	docker run --rm $(TEST_IMAGE) gotestsum --format testdox -- ./cmd/... ./internal/cli/... ./internal/config/... ./internal/process/... ./internal/security/... ./internal/wrap/...

# Run tests with coverage report
test-coverage:
	docker build -f Dockerfile.test -t $(TEST_IMAGE) .
	docker run --rm $(TEST_IMAGE) gotestsum --format testdox -- -cover ./...

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
	docker run --rm $(TEST_IMAGE) go test -bench=BenchmarkShimOverhead -benchtime=10000x -run=^$$ ./internal

# Run benchmark to measure shim overhead on grep (1k iterations, slower command)
benchmark-grep:
	docker build -f Dockerfile.test -t $(TEST_IMAGE) .
	docker run --rm $(TEST_IMAGE) go test -bench=BenchmarkShimOverheadGrep -benchtime=1000x -run=^$$ ./internal

# Run all benchmarks
benchmark-all:
	docker build -f Dockerfile.test -t $(TEST_IMAGE) .
	docker run --rm $(TEST_IMAGE) go test -bench=Benchmark -benchtime=1000x -run=^$$ ./internal

# Run full benchmark with cat (1 million iterations)
benchmark-full:
	docker build -f Dockerfile.test -t $(TEST_IMAGE) .
	docker run --rm $(TEST_IMAGE) go test -bench=BenchmarkShimOverhead -benchtime=1000000x -run=^$$ ./internal

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

# Create a new release
# Usage: make release VERSION=0.1.0-alpha.6
release:
ifndef VERSION
	$(error VERSION is required. Usage: make release VERSION=0.1.0-alpha.6)
endif
	./scripts/release.sh $(VERSION)

clean:
	rm -rf $(BUILD_DIR)
	go clean
