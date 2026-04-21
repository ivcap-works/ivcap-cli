SHELL := /usr/bin/env bash

GIT_COMMIT := $(shell git rev-list --abbrev-commit --tags --max-count=1)
ifeq ($(OS),Windows_NT)
	space := $(subst ,, )
	BUILD_DATE := $(subst $(space),,$(strip $(shell date /T))-$(shell time /T))
	GIT_TAG := $(shell git describe --abbrev=0 --tags 2>/dev/null)
	GOPRIVATE_OS_ENV_CMD := set GOPRIVATE="github.com/ivcap-works/ivcap-core-api" &&
	EXTENSION := .exe
	USER_SHELL := powershell
else
	BUILD_DATE := $(shell date "+%Y-%m-%dT%H:%M")
	GIT_TAG := $(shell git describe --abbrev=0 --tags 2>/dev/null || true)
	GOPRIVATE_OS_ENV_CMD := export GOPRIVATE="github.com/ivcap-works/ivcap-core-api" &&
	USER_SHELL := $(shell echo $$(basename $$SHELL))
endif

MOVIE_SIZE=1280x720 # 640x360
MOVIE_NAME=ivcap-cli.mp4

LD_FLAGS="-X main.version=${GIT_TAG} -X main.commit=${GIT_COMMIT} -X main.date=${BUILD_DATE}"

build: addlicense check build-docs build-dangerously

install: addlicense check install-dangerously completion

build-dangerously:
	@echo "Building IVCAP-CLI..."
	${GOPRIVATE_ENV_CMD} go mod tidy
	go build -ldflags ${LD_FLAGS} -o ivcap ivcap.go

install-dangerously: build-dangerously
	go install -ldflags ${LD_FLAGS} ivcap.go

build-docs:
	go -C doc build -ldflags ${LD_FLAGS} create-docs.go
	rm -f doc/*.md doc/*.1 doc/*.3
	doc/create-docs
	rm -f doc/create-docs

completion:
	@case ${USER_SHELL} in \
		bash) ivcap completion bash -h ;; \
		zsh)  ivcap completion zsh  -h ;; \
		fish) ivcap completion fish -h ;; \
		powershell) ivcap completion powershell -h ;; \
	esac

clean:
	rm ivcap
test:
	go test -v ./...

check:
	@echo "==> go vet"
	go vet ./...
	@echo "==> golangci-lint"
	$(call tool_bin,golangci-lint) run --tests=false ./...
	@echo "==> gocritic"
	$(call tool_bin,gocritic) check -checkTests=false ./...
	@echo "==> staticcheck"
	$(call tool_bin,staticcheck) -tests=false ./...
	@echo "==> gosec"
	$(call tool_bin,gosec) $(GOSEC_FLAGS) ./...
	@# govulncheck exit codes: 0=no vulns, 3=vulns found. Treat 3 as warning by default.
	@code=0; \
	echo "==> govulncheck"; \
	$(call tool_bin,govulncheck) ./... || code=$$?; \
	if [ $$code -eq 0 ]; then exit 0; fi; \
	if [ $$code -eq 3 ]; then \
		if [ "$(GOVULNCHECK_STRICT)" = "true" ]; then \
			echo "govulncheck found vulnerabilities (strict mode enabled)"; \
			exit 3; \
		fi; \
		echo "govulncheck found vulnerabilities (non-fatal; set GOVULNCHECK_STRICT=true to fail)"; \
		exit 0; \
	fi; \
	exit $$code

release: addlicense check build-docs
  # git tag -a v0.4.0 -m "..."
	# export GITHUB_TOKEN=$(cat .github-release-token)
	# or eval $(cat .github-release-token)
	# brew install goreleaser
	goreleaser release --clean

addlicense:
	# go install github.com/google/addlicense@latest
	addlicense -s \
		-c 'Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230' \
		-l apache \
		./**/*.go

mcp-inspector:
	npx @modelcontextprotocol/inspector --config mcp-inspector.config.json --server default-server

mcp-inspector-sse:
	npx @modelcontextprotocol/inspector --config mcp-inspector.config.json --server sse-8077

tv:
	gource -f --title "ivcap-cli" --seconds-per-day 0.1 --auto-skip-seconds 0.1 --bloom-intensity 0.05 \
		--max-user-speed 500 --highlight-users --hide filenames,dirnames --highlight-dirs --multi-sampling

movie:
	gource -${MOVIE_SIZE} --title "ivcap-cli" --seconds-per-day 0.1 --auto-skip-seconds 0.1 --bloom-intensity 0.05 \
		--max-user-speed 500 --highlight-users --hide filenames,dirnames --highlight-dirs --multi-sampling -o - \
	| ffmpeg -y -r 24 -f image2pipe -vcodec ppm -i - -vcodec libx264 -preset ultrafast -pix_fmt yuv420p \
		-crf 1 -threads 2 -bf 0 \
		${MOVIE_NAME}


# Command existence check
command_exists = command -v $(1) >/dev/null 2>&1

# Go's binary install dir. Prefer GOBIN if set, otherwise fall back to GOPATH/bin.
GOBIN_DIR := $(shell go env GOBIN)
ifeq ($(strip $(GOBIN_DIR)),)
	GOBIN_DIR := $(shell go env GOPATH)/bin
endif

# Location of an installed tool binary.
tool_bin = $(GOBIN_DIR)/$(1)$(EXTENSION)

# Prefer checking the actual installed binary (works even with goenv/pyenv shims).
tool_exists = test -x "$(call tool_bin,$(1))"

# govulncheck strict mode: set true to make `make check` fail when vulnerabilities are found.
GOVULNCHECK_STRICT ?= false

# gosec output can be very noisy (e.g. printing every file it checks). Use -terse by default
# to keep output readable while still reporting issues.
GOSEC_FLAGS ?= -terse

# Verify required dev tools are installed (install missing ones)
verify-tools: install-golangci-lint install-go-critic install-staticcheck install-gosec install-govulncheck install-addlicense
	@echo "All required Go tools are installed successfully!"

# Install Go tools
install-tools: verify-tools install-go-tools
	@echo "All Go tools installed successfully!"

# Install golangci-lint
install-golangci-lint:
	@if $(call tool_exists,golangci-lint); then \
		echo "golangci-lint is already installed."; \
	else \
		echo "Installing golangci-lint..."; \
		GOBIN=$(GOBIN_DIR) go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi

install-go-critic:
	@if $(call tool_exists,gocritic); then \
		echo "go-critic is already installed."; \
	else \
		echo "Installing go-critic..."; \
		GOBIN=$(GOBIN_DIR) go install -v github.com/go-critic/go-critic/cmd/gocritic@latest; \
	fi

install-go-tools:
	@if $(call command_exists,go); then \
		echo "go-tools are already installed."; \
	else \
		echo "Installing go-tools..."; \
		go install golang.org/x/tools/...@latest; \
	fi

install-gosec:
	@if $(call tool_exists,gosec); then \
		echo "gosec is already installed."; \
	else \
		echo "Installing gosec..."; \
		GOBIN=$(GOBIN_DIR) go install github.com/securego/gosec/v2/cmd/gosec@latest; \
	fi

install-govulncheck:
	@if $(call tool_exists,govulncheck); then \
		echo "govulncheck is already installed."; \
	else \
		echo "Installing govulncheck..."; \
		GOBIN=$(GOBIN_DIR) go install golang.org/x/vuln/cmd/govulncheck@latest; \
	fi

install-staticcheck:
	@if $(call tool_exists,staticcheck); then \
		echo "staticcheck is already installed."; \
	else \
		echo "Installing staticcheck..."; \
		GOBIN=$(GOBIN_DIR) go install honnef.co/go/tools/cmd/staticcheck@latest; \
	fi

install-addlicense:
	@if $(call tool_exists,addlicense); then \
		echo "addlicense is already installed."; \
	else \
		echo "Installing addlicense..."; \
		GOBIN=$(GOBIN_DIR) go install github.com/nokia/addlicense@latest; \
	fi
