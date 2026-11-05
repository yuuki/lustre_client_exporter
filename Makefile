.PHONY: build test vet complexity clean

build:
	go build -o lustre_client_exporter ./cmd/lustre_client_exporter/

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

# Rank the most complex functions with cccc (https://github.com/moznion/cccc);
# what is analyzed is configured in cccc.toml.
complexity:
	cccc --table --top-cognitive 20 .

clean:
	rm -f lustre_client_exporter
