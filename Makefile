.DEFAULT_GOAL := test

.PHONY: test test-race fmt fmt-check vet staticcheck lint vulncheck check build

test:
	go test ./...

test-race:
	CGO_ENABLED=1 go test -race ./...

fmt:
	gofmt -w .

fmt-check:
	@files="$$(gofmt -l .)" || exit $$?; \
	if [ -n "$$files" ]; then \
		printf 'Run make fmt to format these files:\n%s\n' "$$files"; \
		exit 1; \
	fi

vet:
	go vet ./...

staticcheck:
	go tool staticcheck ./...

lint: fmt-check vet staticcheck

vulncheck:
	go tool govulncheck ./...

check: lint test-race

build:
	mkdir -p bin
	go build -o bin/commitfence ./cmd/commitfence
