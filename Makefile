.PHONY: build build-linux build-all release clean test run deps fmt lint docker-build docker-run

GO ?= go
LDFLAGS := -s -w -buildid=
AGENT_PACKAGE := ./cmd/sentinel

# Build for current platform
build:
	mkdir -p bin
	CGO_ENABLED=0 $(GO) build -trimpath -mod=readonly -ldflags="$(LDFLAGS)" -o bin/sentinel $(AGENT_PACKAGE)

# Build for Linux (for deployment)
build-linux:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -mod=readonly -ldflags="$(LDFLAGS)" -o bin/sentinel-linux-amd64 $(AGENT_PACKAGE)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -mod=readonly -ldflags="$(LDFLAGS)" -o bin/sentinel-linux-arm64 $(AGENT_PACKAGE)

# Build for all platforms
build-all: build build-linux
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build -trimpath -mod=readonly -ldflags="$(LDFLAGS)" -o bin/sentinel-darwin-amd64 $(AGENT_PACKAGE)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build -trimpath -mod=readonly -ldflags="$(LDFLAGS)" -o bin/sentinel-darwin-arm64 $(AGENT_PACKAGE)

# Build clean Linux release artifacts and checksums
release: test
	rm -rf dist
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -mod=readonly -ldflags="$(LDFLAGS)" -o dist/sentinel-linux-amd64 $(AGENT_PACKAGE)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -mod=readonly -ldflags="$(LDFLAGS)" -o dist/sentinel-linux-arm64 $(AGENT_PACKAGE)
	cd dist && shasum -a 256 sentinel-linux-amd64 sentinel-linux-arm64 > SHA256SUMS
	@if LC_ALL=C grep -aqE '/Users/|/home/runner/work/|/github/workspace|/go/pkg/mod' dist/sentinel-linux-*; then \
		echo 'Release artifact contains build-machine paths'; \
		exit 1; \
	fi

# Clean build artifacts
clean:
	rm -rf bin/ dist/

# Run tests
test:
	CGO_ENABLED=0 $(GO) test -v ./...

# Run agent locally
run:
	$(GO) run ./cmd/sentinel

# Install dependencies
deps:
	$(GO) mod tidy
	$(GO) mod download

# Format code
fmt:
	$(GO) fmt ./...

# Lint code
lint:
	golangci-lint run

# Docker build for testing
docker-build:
	docker build -t sitekit-agent:latest .

# Docker run for testing
docker-run:
	docker run --rm -it \
		-e SITEKIT_SAAS_URL=http://host.docker.internal \
		-e SITEKIT_AGENT_TOKEN=test-token \
		sitekit-agent:latest
