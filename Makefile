# kist — see docs/design/ for what any of this is for.
#
# `make check` is the local equivalent of the CI workflow. Run it before
# pushing and the red X never appears.

BIN     := bin/kist
DIST    := dist
PKG     := github.com/jinzhao1994/kist

# The complete SQLite replay race suite can exceed Go's default package deadline.
GO_TEST_TIMEOUT ?= 20m

# Release builds inject the git tag; local builds report "dev" plus the commit
# that the Go toolchain stamps in automatically.
VERSION ?=
LDFLAGS := -X $(PKG)/internal/buildinfo.version=$(VERSION)

# The four platforms v0.1 promises a single binary on.
PLATFORMS := darwin/arm64 darwin/amd64 linux/amd64 windows/amd64

# CGO_ENABLED=0 is load-bearing for build and dist — a binary that quietly
# linked against libc is not the single binary we promise, and the pure-Go
# SQLite driver exists precisely so we never need it. Tests are left on the
# toolchain default: -race needs cgo on some platforms.
GOBUILD := CGO_ENABLED=0 go build -trimpath

.PHONY: all
all: check

.PHONY: check
check: release-check web-check notices-check tidy-check fmt-check vet test build ## Everything CI runs, locally

.PHONY: tidy-check
tidy-check: ## Fail if go.mod/go.sum are not tidy
	go mod tidy -diff

.PHONY: fmt
fmt: ## Rewrite files with gofmt
	gofmt -w .

.PHONY: fmt-check
fmt-check: ## Fail if anything is unformatted
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "not gofmt'd:"; echo "$$unformatted"; exit 1; \
	fi

.PHONY: vet
vet:
	go vet ./...

.PHONY: test
test:
	go test -race -timeout=$(GO_TEST_TIMEOUT) ./...

.PHONY: build
build: web-build ## Build for the host platform into bin/
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o $(BIN) ./cmd/kist

.PHONY: dist
dist: web-build notices-check ## Cross-compile every release platform into dist/
	@rm -rf $(DIST) && mkdir -p $(DIST)
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		out=$(DIST)/kist_$${os}_$${arch}; \
		if [ "$$os" = "windows" ]; then out=$$out.exe; fi; \
		echo "  $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch $(GOBUILD) \
			-ldflags '$(LDFLAGS) -s -w' -o $$out ./cmd/kist || exit 1; \
	done
	python3 scripts/package-release.py
	@ls -lh $(DIST)

.PHONY: clean
clean:
	rm -rf $(BIN) $(DIST)

.PHONY: help
help: ## List targets
	@grep -hE '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk -F':.*?## ' '{printf "  %-12s %s\n", $$1, $$2}'

.PHONY: web-build web-check
web-build: ## Build the embedded browser app
	npm --prefix web run build

web-check: ## Check generated API types, browser tests, and production assets
	npm --prefix web run check

.PHONY: release-check
release-check: ## Check container publication validation
	python3 scripts/test-container-release.py
	python3 scripts/test-container-channel.py
	python3 scripts/test-container-mirror.py
	python3 scripts/test-package-release.py

.PHONY: notices-check
notices-check: ## Verify third-party notices against linked production dependencies
	python3 scripts/third-party-notices.py --check
