.PHONY: build test vet clean

build:
	go build -o lustre_client_exporter ./cmd/lustre_client_exporter/

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

clean:
	rm -f lustre_client_exporter
