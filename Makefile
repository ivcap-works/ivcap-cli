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
	go build -ldflags ${LD_FLAGS} ivcap.go

install-dangerously: build-dangerously
	go install -ldflags ${LD_FLAGS} ivcap.go

build-docs:
	go -C doc build -ldflags ${LD_FLAGS} create-docs.go
	rm -f doc/*.md doc/*.3
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
	go vet ./...
	golangci-lint run  --out-format line-number --tests=false ./...
	gocritic check -checkTests=false ./...
	staticcheck -tests=false ./...
	gosec ./...
	govulncheck ./...

release: addlicense check build-docs
  # git tag -a v0.4.0 -m "..."
	# export GITHUB_TOKEN=$(cat .github-release-token)
	# or eval $(cat .github-release-token)
	# brew install goreleaser
	goreleaser release --clean

addlicense:
	# go install github.com/google/addlicense@latest
	addlicense -v \
		-c 'Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230' \
		-l apache \
		./**/*.go

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

# Install Go tools
install-tools: install-golangci-lint install-go-critic install-go-tools install-gosec install-govulncheck install-staticcheck install-addlicense
	@echo "All Go tools installed successfully!"

# Install golangci-lint
install-golangci-lint:
	@if $(call command_exists,golangci-lint); then \
		echo "golangci-lint is already installed."; \
	else \
		echo "Installing golangci-lint..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin; \
	fi

install-go-critic:
	@if $(call command_exists,gocritic); then \
		echo "go-critic is already installed."; \
	else \
		echo "Installing go-critic..."; \
		go install -v github.com/go-critic/go-critic/cmd/gocritic@latest; \
	fi

install-go-tools:
	@if $(call command_exists,go); then \
		echo "go-tools are already installed."; \
	else \
		echo "Installing go-tools..."; \
		go install golang.org/x/tools/...@latest; \
	fi

install-gosec:
	@if $(call command_exists,gosec); then \
		echo "gosec is already installed."; \
	else \
		echo "Installing gosec..."; \
		go install github.com/securego/gosec/v2/cmd/gosec@latest; \
	fi

install-govulncheck:
	@if $(call command_exists,govulncheck); then \
		echo "govulncheck is already installed."; \
	else \
		echo "Installing govulncheck..."; \
		go install golang.org/x/vuln/cmd/govulncheck@latest; \
	fi

install-staticcheck:
	@if $(call command_exists,staticcheck); then \
		echo "staticcheck is already installed."; \
	else \
		echo "Installing staticcheck..."; \
		go install honnef.co/go/tools/cmd/staticcheck@latest; \
	fi

install-addlicense:
	@if $(call command_exists,addlicense); then \
		echo "addlicense is already installed."; \
	else \
		echo "Installing addlicense..."; \
		go get -u github.com/nokia/addlicense; \
	fi

mcp-inspector:
	npx @modelcontextprotocol/inspector --config mcp-inspector.config.json --server default-server

mcp-inspector-sse:
	npx @modelcontextprotocol/inspector --config mcp-inspector.config.json --server sse-8077
