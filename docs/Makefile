# Minimal Makefile for ivcap-cli documentation
#
# The command reference (ivcap*.md) and man pages (../man/) are generated
# automatically from the Go CLI source via cobra's doc package.
# Hand-authored pages (index.md, overview.md, etc.) are not regenerated.
#
# Docs are rendered with MkDocs Material via Docker – no local Python install needed.
# Requires: docker

PORT                ?= 8000
DOCS_IMAGE          ?= squidfunk/mkdocs-material
DOCS_CONTAINER_NAME ?= ivcap-cli-docs

.PHONY: help generate serve stop build clean

help:
	@echo "Available targets:"
	@echo "  generate          Rebuild command reference from cobra (delegates to root Makefile)"
	@echo "  serve  [PORT=…]   Live-reload dev server via Docker at http://localhost:\$${PORT}"
	@echo "  stop              Stop a running docs server container"
	@echo "  build             Build static HTML site into ../site/"
	@echo "  clean             Remove generated command-reference files and ../site/"

generate:
	$(MAKE) -C .. build-docs

serve: stop
	docker run --rm --name $(DOCS_CONTAINER_NAME) \
		-p $(PORT):8000 \
		-v "$(shell pwd)/..":/docs \
		$(DOCS_IMAGE) serve --dev-addr 0.0.0.0:8000

stop:
	@docker rm -f $(DOCS_CONTAINER_NAME) 2>/dev/null || true
	@docker ps -q --filter "publish=$(PORT)" | xargs -r docker stop 2>/dev/null || true

build:
	docker run --rm \
		-v "$(shell pwd)/..":/docs \
		$(DOCS_IMAGE) build --clean

clean:
	rm -f ivcap*.md
	rm -f ../man/*.1 ../man/*.3
	rm -rf ../site
