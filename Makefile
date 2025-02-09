# 
# 
REPO:=macrbg
PROG:=external-mdns
VERSION:=$(shell git describe --tags --abbrev=0 --always)
BUILD_DATE:= $(shell date +%Y-%m-%d)	
GIT_COMMIT := $(shell git rev-parse --short=8 HEAD)

GO ?= go
GO_SRC := $(shell find . -name '*.go' -not -path "./vendor/*")

#
IMAGE_NAME:=$(REPO)/$(PROG)
IMAGE_VTAG:=$(VERSION)
IMAGE_TAG:=$(GIT_COMMIT)

native:  $(GO_SRC) ## Build a native binary
	$(GO) build -o bin/$(PROG) .

docker:  ## Build a docker image
	docker buildx build --platform linux/amd64 -t $(IMAGE_NAME):$(IMAGE_TAG) -f Dockerfile .