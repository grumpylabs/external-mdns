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

arm64: $(GO_SRC) ## Build an ARM64 binary
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 ${GO} build -o bin/$(PROG)-linux_arm64 .

pkg-arm64: arm64 ## Package ARM64 binary
	docker build --platform linux/arm64 -t $(IMAGE_NAME)-arm64:$(IMAGE_TAG) --provenance=false -f Dockerfile .
	docker push $(IMAGE_NAME)-arm64:$(IMAGE_TAG)

amd64: $(GO_SRC) ## Build an AMD64 binary
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 ${GO} build -o bin/$(PROG)-linux_amd64 .

pkg-amd64: amd64 ## Package AMD64 binary
	docker build --platform linux/amd64 -t $(IMAGE_NAME)-amd64:$(IMAGE_TAG) --provenance=false -f Dockerfile .
	docker push $(IMAGE_NAME)-amd64:$(IMAGE_TAG)

docker: pkg-arm64 pkg-amd64 ## Build a docker image
	docker manifest rm $(IMAGE_NAME):$(IMAGE_TAG) || true
	docker manifest create $(IMAGE_NAME):$(IMAGE_TAG) \
		$(IMAGE_NAME)-amd64:$(IMAGE_TAG) \
		$(IMAGE_NAME)-arm64:$(IMAGE_TAG)

push: docker ## Push a docker manifest image		
	docker manifest push $(IMAGE_NAME):$(IMAGE_TAG)

clean: ## Clean up
	rm -rf bin

help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "}; /^[a-zA-Z0-9_-]+:.*##/ {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort

.DEFAULT_GOAL := help