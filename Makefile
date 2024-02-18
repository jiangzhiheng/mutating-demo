export  DATE:=${shell date "+%Y%m%d%H%M"}
REGISTRY ?= jiangzhiheng
IMAGE    ?= $(REGISTRY)/mutating-demo
VERSION  ?= v0.1-${DATE}

# container builds a Docker image.
.PHONY: container
container:
	docker build --platform=linux/amd64 -t $(IMAGE):$(VERSION) .
	docker push $(IMAGE):$(VERSION)