.PHONY: all build-agent build-collector build-cli test clean generate-bpf generate-proto help

all: build-agent build-collector build-cli

build-agent:
	@echo "Building GreenToken Agent..."
	cd agent && go build -o ../bin/greentoken-agent .

build-collector:
	@echo "Building GreenToken Collector..."
	cd collector && go build -o ../bin/greentoken-collector .

build-cli:
	@echo "Building GreenToken CLI..."
	cd cmd/greentoken && go build -o ../../bin/greentoken .

test:
	@echo "Running tests..."
	go test ./agent/... ./collector/...

generate-bpf:
	@echo "Generating BPF code..."
	cd agent && go generate ./...

generate-proto:
	@echo "Generating Protobuf files..."
	protoc --go_out=. --go-grpc_out=. pb/greentoken.proto

clean:
	@echo "Cleaning binaries..."
	rm -rf bin/

help:
	@echo "Available targets:"
	@echo "  build-agent      Build the GreenToken agent binary"
	@echo "  build-collector  Build the GreenToken collector binary"
	@echo "  test             Run all tests in the workspace"
	@echo "  generate-bpf     Generate BPF code using bpf2go"
	@echo "  generate-proto   Generate Go files from protobuf definition"
	@echo "  clean            Remove built binaries"
