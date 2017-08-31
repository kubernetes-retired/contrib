# Copyright 2016 The Kubernetes Authors. All rights reserved
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

all: build

FLAGS =
ENVVAR = GOOS=linux GOARCH=amd64 CGO_ENABLED=0
REGISTRY = gcr.io/google_containers
TAG = v0.0.1

deps:
	go get github.com/tools/godep

build: clean deps
	$(ENVVAR) godep go build ./...
	$(ENVVAR) godep go build -o check_metadata_concealment

container: build
	docker build --pull -t ${REGISTRY}/check-metadata-concealment:$(TAG) .

push: container
	gcloud docker -- push ${REGISTRY}/check-metadata-concealment:$(TAG)

clean:
	rm -f check_metadata_concealment

.PHONY: all deps build container push clean
