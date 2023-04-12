
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

MOVIE_SIZE=1280x720 # 640x360
MOVIE_NAME=ivcap-cli.mp4


build:
	@echo "Building IVCAP-CLI..."
	${GOPRIVATE_ENV_CMD} go mod tidy
	go build -ldflags "-X main.version=${GIT_TAG} -X main.commit=${GIT_COMMIT} -X main.date=${BUILD_DATE}" -o ivcap${EXTENSION}

install: addlicense build 
	cp ivcap $(shell go env GOBIN)
	if [[ -d $(shell brew --prefix)/share/zsh/site-functions ]]; then \
		$(shell go env GOBIN)/ivcap completion zsh > $(shell brew --prefix)/share/zsh/site-functions/_ivcap;\
  fi

test:
	go test -v ./...

release: addlicense
  # git tag -a v0.4.0 -m "..."
	# export GITHUB_TOKEN=$(cat .github-release-token)
	# or eval $(cat .github-release-token)
	# brew install goreleaser
	goreleaser release --rm-dist

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
