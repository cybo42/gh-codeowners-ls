BINARY := gh-codeowners-ls
GOFILES := $(shell find . -name '*.go' -not -path './vendor/*')

.DEFAULT_GOAL := all

.PHONY: all
all: fmt vet test build           ## fmt, vet, test, then build

.PHONY: build
build: $(BINARY)                  ## build the extension binary

$(BINARY): $(GOFILES) go.mod go.sum
	go build -o $(BINARY) .

.PHONY: install
install: build                    ## build + install as a local gh extension
	gh extension remove codeowners-ls 2>/dev/null || true
	gh extension install .
	@echo "installed — try: gh codeowners-ls --help"

.PHONY: test
test:                             ## run unit tests with race detector
	go test -race ./...

.PHONY: cover
cover:                            ## test with coverage report
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

.PHONY: fmt
fmt:                              ## check formatting (fails, does not rewrite)
	@out="$$(gofmt -l .)"; \
	if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

.PHONY: vet
vet:                              ## go vet
	go vet ./...

.PHONY: lint
lint:                             ## golangci-lint, if installed
	@command -v golangci-lint >/dev/null \
		&& golangci-lint run \
		|| echo "golangci-lint not installed, skipping"

.PHONY: tidy
tidy:                             ## verify go.mod/go.sum are tidy
	go mod tidy -diff

.PHONY: check
check: fmt vet tidy test lint     ## everything CI would run

.PHONY: smoke
smoke: build                      ## run against this repo as a sanity check
	./$(BINARY) --help
	./$(BINARY) --json >/dev/null && echo "smoke ok"

.PHONY: dist
dist:                             ## cross-compile via script/build.sh
	./script/build.sh

.PHONY: clean
clean:
	rm -f $(BINARY) coverage.out
	rm -rf dist/

.PHONY: help
help:                             ## list targets
	@grep -hE '^[a-z-]+:.*##' $(MAKEFILE_LIST) \
		| sed 's/:.*##/\t/' | expand -t20
