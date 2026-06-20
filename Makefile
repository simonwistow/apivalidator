.PHONY: build test lint vet fuzz clean

BINARY := apivalidator
CMD     := ./cmd/apivalidator

build:
	go build -o $(BINARY) $(CMD)

test:
	go test -race ./...

test-cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run ./...

vet:
	go vet ./...

fuzz:
	go test -fuzz=FuzzValidateRequest -fuzztime=30s ./...

clean:
	rm -f $(BINARY) coverage.out coverage.html
	rm -rf dist/
