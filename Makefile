BIN := ./bin/milvus-health

.PHONY: fmt test build run-help clean

fmt:
	gofmt -w $(shell find . -name '*.go' -not -path './bin/*')

test:
	go test ./...

build:
	mkdir -p ./bin
	go build -o $(BIN) .

run-help: build
	$(BIN) version
	$(BIN) check --help
	$(BIN) validate --help

clean:
	rm -rf ./bin
