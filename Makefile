
GIT_COMMIT := $(shell git rev-list --abbrev-commit --tags --max-count=1)
BUILD_DATE := $(shell date "+%Y-%m-%dT%H:%M")
GIT_TAG := $(shell git describe --abbrev=0 --tags ${TAG_COMMIT} 2>/dev/null || true)
ifeq ($(GIT_TAG),)
VERSION := ${GIT_COMMIT}|${BUILD_DATE}
else
VERSION := $(GIT_TAG:v%=%)|${GIT_COMMIT}|${BUILD_DATE}
endif

build:
	go mod tidy
	go build -ldflags "-X main.version=${GIT_TAG} -X main.commit=${GIT_COMMIT} -X main.date=${BUILD_DATE}" -o ivcap

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
