.PHONY: all build test test-race cover lint fmt vet tidy examples integration bench check

all: check

build:
	go build ./...

test:
	go test -count=1 ./...

test-race:
	go test -race -count=1 ./...

cover:
	go test -covermode=atomic -coverprofile=coverage.out .
	go tool cover -func=coverage.out | tail -1

lint:
	golangci-lint run ./...

fmt:
	gofmt -l -w .

vet:
	go vet ./...

tidy:
	go mod tidy

examples:
	go build ./examples/...

integration:
	go test -tags=integration -count=1 ./...

bench:
	go test -bench=. -benchmem -run=^$$ ./...

check: fmt vet build test-race lint
