# Check to see if we can use ash, in Alpine images, or default to BASH.
SHELL_PATH = /bin/ash
SHELL = $(if $(wildcard $(SHELL_PATH)),/bin/ash,/bin/bash)

-include environ.inc
.PHONY: deps dev build install image release test clean tr tr-merge

export CGO_ENABLED=0
VERSION=$(shell git describe --abbrev=0 --tags 2>/dev/null || echo "$VERSION")
COMMIT=$(shell git rev-parse --short HEAD || echo "$COMMIT")
BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
GOCMD=go
GOVER=$(shell go version | grep -o -E 'go1\.17\.[0-9]+')

DESTDIR=/usr/local/bin

ifeq ($(BRANCH), master)
IMAGE := zacksfF/PubSubGo
TAG := latest
else
IMAGE := zacksfF/PubSubGo
TAG := dev
endif

all: preflight build

preflight:
	@./script/version.sh

deps:

dev : DEBUG=1
dev : build
	@./PubSubGo -v
	@./PubSubGo -v

cli:
	@$(GOCMD) build $(FLAGS) -tags "netgo static_build" -installsuffix netgo \
		-ldflags "-w \
		-X $(shell go list).Version=$(VERSION) \
		-X $(shell go list).Commit=$(COMMIT)" \
		./cmd/pubsub/

server: generate
	@$(GOCMD) build $(FLAGS) -tags "netgo static_build" -installsuffix netgo \
		-ldflags "-w \
		-X $(shell go list).Version=$(VERSION) \
		-X $(shell go list).Commit=$(COMMIT)" \
		./cmd/used/...

build: cli server

generate:
	@if [ x"$(DEBUG)" = x"1"  ]; then		\
	  echo 'Running in debug mode...';	\
	fi

install: build
	@install -D -m 755 used $(DESTDIR)/used
	@install -D -m 755 pubsub $(DESTDIR)/pubsub

ifeq ($(PUBLISH), 1)
image: generate
	@docker build --build-arg VERSION="$(VERSION)" --build-arg COMMIT="$(COMMIT)" -t $(IMAGE):$(TAG) .
	@docker push $(IMAGE):$(TAG)
else
image: generate
	@docker build --build-arg VERSION="$(VERSION)" --build-arg COMMIT="$(COMMIT)" -t $(IMAGE):$(TAG) .
endif

release: generate
	@./script/release.sh

fmt:
	@$(GOCMD) fmt ./...

test:
	@CGO_ENABLED=1 $(GOCMD) test -v -cover -race ./...
