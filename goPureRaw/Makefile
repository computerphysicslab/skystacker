.PHONY: all fmt vet test fuzz lint clean

all: fmt vet test

fmt:
	gofmt -l -w .

vet:
	go vet ./...

test:
	CGO_ENABLED=0 go test ./...

fuzz:
	@echo "no fuzz targets defined yet"

lint:
	golangci-lint run

clean:
	go clean -testcache
