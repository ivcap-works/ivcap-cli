SHELL := /usr/bin/env bash

GIT_COMMIT := $(shell git rev-list --abbrev-commit --tags --max-count=1)
ifeq ($(OS),Windows_NT)
	space := $(subst ,, )
	BUILD_DATE := $(subst $(space),,$(strip $(shell date /T))-$(shell time /T))
	GIT_TAG := $(shell git describe --abbrev=0 --tags 2> nul)
	GOPRIVATE_OS_ENV_CMD := set GOPRIVATE="github.com/ivcap-works/ivcap-core-api" &&
	EXTENSION := .exe
else
	BUILD_DATE := $(shell date "+%Y-%m-%dT%H:%M")
	GIT_TAG := $(shell git describe --abbrev=0 --tags 2>/dev/null || true)
	GOPRIVATE_OS_ENV_CMD := export GOPRIVATE="github.com/ivcap-works/ivcap-core-api" &&
endif

UNAME := $(shell sh -c 'uname 2>/dev/null || echo Unknown')
USER_SHELL := $(shell echo $$SHELL)

MOVIE_SIZE=1280x720 # 640x360
MOVIE_NAME=ivcap-cli.mp4

LD_FLAGS="-X main.version=${GIT_TAG} -X main.commit=${GIT_COMMIT} -X main.date=${BUILD_DATE}"

build: addlicense check build-docs build-dangerously

install: addlicense check install-dangerously

build-dangerously:
	@echo "Building IVCAP-CLI..."
	${GOPRIVATE_ENV_CMD} go mod tidy
	go build -ldflags ${LD_FLAGS} ivcap.go

install-dangerously:
	go install -ldflags ${LD_FLAGS} ivcap.go

build-docs:
	go -C doc build -ldflags ${LD_FLAGS} create-docs.go
	rm -f doc/*.md doc/*.3
	doc/create-docs
	rm -f doc/create-docs

ifeq ($(UNAME), Linux)
completion: completion-linux
else ifeq ($(UNAME), Darwin)
completion: completion-mac
else
completion:
	@echo "Unsupported operating system. Cannot add IVCAP completion."
endif

completion-mac:
	@if [[ -d $(shell brew --prefix)/share/zsh/site-functions ]]; then \
		echo "Adding completion for ZSH." ;\
		$(shell go env GOBIN)/ivcap completion zsh > $(shell brew --prefix)/share/zsh/site-functions/_ivcap ;\
	else \
		echo "Unsupported macOS shell. Cannot add IVCAP completion." ;\
	fi

completion-linux:
	@if [ "${USER_SHELL}" = "/usr/bin/zsh" ] && [[ -d "${HOME}/.zsh/" ]] ; then \
		echo -e '\nAdding ZSH completions to "${HOME}/.zsh/site-functions".' ;\
		mkdir -p "${HOME}/.zsh/site-functions/" ;\
		$(shell go env GOPATH)/bin/ivcap completion zsh > "${HOME}/.zsh/site-functions/_ivcap" ;\
		echo -e 'Please add "${HOME}/.zsh/site-functions" to your .zshrc config:\n' ;\
		echo -e '    fpath=("$${HOME}/.zsh/site-functions" $$fpath).\n' ;\
	else \
		echo "Unsupported Linux shell. Cannot add IVCAP completion." ;\
	fi

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
	# go install github.com/google/addlicense@v1.0.0
	addlicense -c 'Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230' -l apache ./**/*.go

tv:
	gource -f --title "ivcap-cli" --seconds-per-day 0.1 --auto-skip-seconds 0.1 --bloom-intensity 0.05 \
		--max-user-speed 500 --highlight-users --hide filenames,dirnames --highlight-dirs --multi-sampling

movie:
	gource -${MOVIE_SIZE} --title "ivcap-cli" --seconds-per-day 0.1 --auto-skip-seconds 0.1 --bloom-intensity 0.05 \
		--max-user-speed 500 --highlight-users --hide filenames,dirnames --highlight-dirs --multi-sampling -o - \
	| ffmpeg -y -r 24 -f image2pipe -vcodec ppm -i - -vcodec libx264 -preset ultrafast -pix_fmt yuv420p \
		-crf 1 -threads 2 -bf 0 \
		${MOVIE_NAME}
