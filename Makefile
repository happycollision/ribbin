.PHONY: build install test clean

BINARY_NAME=ribbin
BUILD_DIR=bin

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/ribbin

install: build
	go install ./cmd/ribbin

test:
	go test ./...

clean:
	rm -rf $(BUILD_DIR)
	go clean
