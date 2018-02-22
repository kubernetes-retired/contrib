# Makefile for the Docker image proxy-to-service
# MAINTAINER: Tim Hockin <thockin@google.com>
# If you update this image please bump the tag value before pushing.

.PHONY: all container push

TAG = v2
PREFIX = staging-k8s.gcr.io
NAME = proxy-to-service

all: container

container:
	docker build --pull -t $(PREFIX)/$(NAME):$(TAG) .

push:
	gcloud docker -- push $(PREFIX)/$(NAME):$(TAG)
