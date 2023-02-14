
GIT_COMMIT := $(shell git rev-list --abbrev-commit --tags --max-count=1)
ifeq ($(OS),Windows_NT)
	space := $(subst ,, )
	BUILD_DATE := $(subst $(space),,$(strip $(shell date /T))-$(shell time /T))
	GIT_TAG := $(shell git describe --abbrev=0 --tags 2> nul)
	GOPRIVATE_OS_ENV_CMD := set GOPRIVATE="github.com/reinventingscience/ivcap-core-api" &&
	EXTENSION := .exe
else
	BUILD_DATE := $(shell date "+%Y-%m-%dT%H:%M")
	GIT_TAG := $(shell git describe --abbrev=0 --tags 2>/dev/null || true)
	GOPRIVATE_OS_ENV_CMD := export GOPRIVATE="github.com/reinventingscience/ivcap-core-api" &&
endif

ifeq ($(GIT_TAG),)
	VERSION := "${GIT_COMMIT}${BUILD_DATE}"
else
	VERSION := "$(GIT_TAG)|${GIT_COMMIT}|${BUILD_DATE}"
endif

build:
	@echo "Building IVCAP-CLI..."
	${GOPRIVATE_ENV_CMD} go mod tidy
	go build -ldflags "-X main.version=${GIT_TAG} -X main.commit=${GIT_COMMIT} -X main.date=${BUILD_DATE}" -o ivcap${EXTENSION}

install: build
	cp ivcap $(shell go env GOBIN)
#	go install -ldflags "-X main.Version=${VERSION}" -o ivcap

test:
	go test -v ./...

release:
  # git tag -a v0.4.0 -m "..."
	# export GITHUB_TOKEN=$(cat .github-release-token)
	# or eval $(cat .github-release-token)
	# brew install goreleaser
	goreleaser release --rm-dist

addlicense:
	# go install github.com/google/addlicense@v1.0.0
	addlicense -c 'Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230' -l apache ./**/*.go