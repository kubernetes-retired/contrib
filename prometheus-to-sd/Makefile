# Copyright 2017 The Kubernetes Authors. All rights reserved#
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
# limitations under the License..

all: build

ENVVAR = GOOS=linux GOARCH=amd64 CGO_ENABLED=0
PREFIX = gcr.io/google-containers
TAG = v0.1.0

build:
	$(ENVVAR) godep go build -a -o monitor

test:
	$(ENVVAR) godep go test ./...

container: build
	docker build --pull -t ${PREFIX}/prometheus-to-sd:$(TAG) .

push: container
	gcloud docker -- push ${PREFIX}/prometheus-to-sd:$(TAG)

clean:
	rm -rf monitor
