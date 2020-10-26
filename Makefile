RSS_IMAGE_NAME ?= "something-rss"
RSS_IMAGE_TAG ?= "latest"

.PHONY: build-rss
build-rss:
	docker build -t $(RSS_IMAGE_NAME):$(RSS_IMAGE_TAG) -f rss/Dockerfile .
