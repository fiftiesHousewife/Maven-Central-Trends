.PHONY: build run test lint clean export

build:
	go build -o bin/server ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./...

lint:
	golangci-lint run

export:
	go run ./cmd/export docs/site

clean:
	rm -rf bin/ docs/site/
